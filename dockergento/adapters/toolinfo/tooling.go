// Package toolinfo answers what the machine says about itself: which versions, and whether Xdebug
// is on.
package toolinfo

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/dockerd"
)

// Reader reads them.
type Reader struct {
	// Root is the checkout of a source build, whose git describe is its version.
	Root string

	// Stamped is the version a release binary was built with. When it is set, Version is it and
	// git is not asked.
	Stamped string

	// WorkdirPHP is where the code lives inside the container.
	WorkdirPHP string
}

// Version is the tool's own version, from git rather than from a file: a file is a second place
// to remember to change, and it is always the one that is wrong.
//
// `git describe --tags` without --abbrev=0, as `hm version` reads it: rounded to the nearest tag, a
// build commits past it was described and labelled as that older release.
func (r Reader) Version() string {
	if r.Stamped != "" {
		return r.Stamped
	}

	command := exec.Command("git", "describe", "--tags")
	command.Dir = r.Root

	output, err := command.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// ComposeVersion is still a subprocess, and will stay one: Compose is a CLI plugin, so its
// version is not something the daemon can be asked.
func (Reader) ComposeVersion() string {
	output, err := exec.Command("docker", "compose", "version", "--short").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func (r Reader) Workdir() string {
	if r.WorkdirPHP != "" {
		return r.WorkdirPHP
	}

	return os.Getenv("WORKDIR_PHP")
}

// Xdebug reports whether the extension is loaded in the running php container.
//
// It is the one thing here that has to look inside a container. With the API that is an exec,
// which is why it answers "unknown" rather than guessing when there is nothing to look in — a
// stopped environment has no opinion about Xdebug.
func (Reader) Xdebug(project string) string {
	docker, err := connect()
	if err != nil {
		return "unknown"
	}
	defer docker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return "unknown"
	}

	id := phpfpmOf(containers, project)
	if id == "" {
		return "unknown"
	}

	created, err := docker.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd: []string{"grep", "-q", "^zend_extension",
			"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "unknown"
	}

	attached, err := docker.ContainerExecAttach(ctx, created.ID, container.ExecStartOptions{})
	if err != nil {
		return "unknown"
	}
	attached.Close()

	inspected, err := docker.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return "unknown"
	}

	if inspected.ExitCode == 0 {
		return "on"
	}

	return "off"
}

// phpfpmOf is the id of the project's php container among those running, or "" when it has none.
//
// The label to read is chosen before comparing, as core.Container.Key() does: compared label by
// label, an empty name matched every container that has no hm.project.
func phpfpmOf(containers []container.Summary, project string) string {
	if project == "" {
		return ""
	}

	for _, item := range containers {
		key := item.Labels["hm.project"]
		if key == "" {
			key = item.Labels["com.docker.compose.project"]
		}

		if key == project && item.Labels["com.docker.compose.service"] == "phpfpm" {
			return item.ID
		}
	}

	return ""
}

func connect() (*client.Client, error) {
	options := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	if host := dockerd.Endpoint(); host != "" {
		options = append(options, client.WithHost(host))
	}

	return client.NewClientWithOpts(options...)
}

// State reads whether this copy of the data has been anonymised.
type State struct {
	// Dir is where that is recorded, outside any checkout.
	Dir string
}

// Anonymisation reports "yes" or "unknown", and when.
//
// Unknown is the honest answer for a project nobody has touched, and it is never treated as safe:
// a reassuring "yes" left over from before an import would be worse than no record at all.
func (s State) Anonymisation(project string) (string, string) {
	directory := s.Dir
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "unknown", ""
		}

		directory = filepath.Join(home, ".hm", "state")
	}

	contents, err := os.ReadFile(filepath.Join(directory, project+".json"))
	if err != nil {
		return "unknown", ""
	}

	var state struct {
		AnonymisedAt string `json:"anonymised_at"`
	}

	if err := json.Unmarshal(contents, &state); err != nil || state.AnonymisedAt == "" {
		return "unknown", ""
	}

	return "yes", state.AnonymisedAt
}

// Record writes down that this copy of the data was anonymised. The state file may hold other
// keys, which anonymisation.sh kept: the date is merged into them, not written over them.
func (s State) Record(environment, at string) error {
	return s.update(environment, func(document map[string]json.RawMessage) bool {
		value, _ := json.Marshal(at)
		document["anonymised_at"] = value

		return true
	})
}

// Clear is called by everything that replaces the database. Whatever it brought in, nobody
// anonymised it, and a reassuring "yes" left over from before is worse than no record at all.
// Only the anonymisation's key goes; the rest of the file stays.
func (s State) Clear(environment string) error {
	return s.update(environment, func(document map[string]json.RawMessage) bool {
		if _, recorded := document["anonymised_at"]; !recorded {
			return false
		}

		delete(document, "anonymised_at")

		return true
	})
}

// update reads the state file (a missing or unreadable one is empty, as the shell's `|| echo
// '{}'` had it), lets change edit it, and writes it back atomically when change says it changed.
func (s State) update(environment string, change func(map[string]json.RawMessage) bool) error {
	file, err := s.file(environment)
	if err != nil {
		return err
	}

	document := map[string]json.RawMessage{}
	if contents, err := os.ReadFile(file); err == nil {
		if json.Unmarshal(contents, &document) != nil || document == nil {
			document = map[string]json.RawMessage{}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if !change(document) {
		return nil
	}

	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".*")
	if err != nil {
		return err
	}

	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		temporary.Close()
		os.Remove(temporary.Name())

		return err
	}

	if err := temporary.Close(); err != nil {
		os.Remove(temporary.Name())

		return err
	}

	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		os.Remove(temporary.Name())

		return err
	}

	if err := os.Rename(temporary.Name(), file); err != nil {
		os.Remove(temporary.Name())

		return err
	}

	return nil
}

func (s State) file(environment string) (string, error) {
	directory := s.Dir
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		directory = filepath.Join(home, ".hm", "state")
	}

	return filepath.Join(directory, environment+".json"), nil
}

// DockerVersion is the daemon's version, asked of the daemon.
//
// Not being able to answer is a fact worth reporting rather than a reason to fail: a report that
// says "docker: not available" is more useful than no report, and the reason somebody is running
// this command may be that Docker is not running.
func (Reader) DockerVersion() string {
	docker, err := connect()
	if err != nil {
		return ""
	}
	defer docker.Close()

	version, err := docker.ServerVersion(context.Background())
	if err != nil {
		return ""
	}

	return version.Version
}

// ComposeCommand is how Compose is invoked on this machine, which is what a bug report needs to
// tell a v2 plugin from the old standalone one.
func (Reader) ComposeCommand() string {
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		return "docker compose"
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose"
	}

	return ""
}
