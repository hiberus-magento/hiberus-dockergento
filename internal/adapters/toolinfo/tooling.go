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

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/dockerd"
)

// Reader reads them.
type Reader struct {
	// Root is the tool's own directory, whose git describe is its version.
	Root string

	// WorkdirPHP is where the code lives inside the container.
	WorkdirPHP string
}

// Version is the tool's own version, from git rather than from a file: a file is a second place
// to remember to change, and it is always the one that is wrong.
func (r Reader) Version() string {
	command := exec.Command("git", "describe", "--tags", "--abbrev=0")
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

	id := ""
	for _, item := range containers {
		matches := item.Labels["hm.project"] == project ||
			(item.Labels["hm.project"] == "" && item.Labels["com.docker.compose.project"] == project)

		if matches && item.Labels["com.docker.compose.service"] == "phpfpm" {
			id = item.ID

			break
		}
	}

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
