package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

//
// A project on disk, which is what every one of these needs first.
//
// Built rather than copied from a fixture directory: what a test is about is usually one thing
// about the project — that it has a database, or a git history, or is routed through the proxy —
// and a builder says which of those in a line.
//

// Project is a throwaway project this tool can be run in.
type Project struct {
	// Name is its compose project name, and Root is where it lives.
	Name string
	Root string
}

// Compose is the file a project runs on, for a test that wants a particular one.
type Compose struct {
	// Services is the body of the compose file, without the `services:` key.
	Services string

	// Properties are added to the project's own, over the defaults.
	Properties map[string]string
}

// NewProject writes a project and answers where it is.
//
// The three files are what every project of this tool has: the compose file and the two machine
// overlays, which are copies of it here because what varies between them is not what these tests
// are about.
func NewProject(t *testing.T, name string, compose Compose) Project {
	t.Helper()

	root := filepath.Join(t.TempDir(), name)

	if err := os.MkdirAll(filepath.Join(root, "config", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}

	services := compose.Services
	if services == "" {
		services = defaultServices
	}

	contents := "services:\n" + services

	for _, file := range []string{
		"docker-compose.yml",
		"docker-compose.dev.mac.yml",
		"docker-compose.dev.linux.yml",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	properties := map[string]string{
		"MAGENTO_DIR":          ".",
		"DOMAIN":               name + ".invalid",
		"COMPOSE_PROJECT_NAME": name,
	}

	for key, value := range compose.Properties {
		properties[key] = value
	}

	document := "{"
	first := true

	for key, value := range properties {
		if !first {
			document += ","
		}

		document += fmt.Sprintf("%q: %q", key, value)
		first = false
	}

	document += "}\n"

	if err := os.WriteFile(filepath.Join(root, "config", "docker", "properties.json"),
		[]byte(document), 0o644); err != nil {
		t.Fatal(err)
	}

	return Project{Name: name, Root: root}
}

// defaultServices is a php container that is a shell and nothing else: what most of these need is
// somewhere to run a command, not a Magento.
const defaultServices = `  phpfpm:
    image: alpine:latest
    working_dir: /var/www/html
    command: ["sleep", "600"]
`

// Committed makes the project a git repository with everything in it committed, which is what a
// worktree needs to exist at all.
func (p Project) Committed(t *testing.T) Project {
	t.Helper()

	for _, args := range [][]string{
		{"init", "-q", "."},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "inicial"},
	} {
		command := exec.Command("git", args...)
		command.Dir = p.Root

		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	return p
}

// Write puts a file in the project.
func (p Project) Write(t *testing.T, name, contents string) {
	t.Helper()

	path := filepath.Join(p.Root, name)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Read is a file of the project, or the empty string when there is none.
func (p Project) Read(name string) string {
	contents, err := os.ReadFile(filepath.Join(p.Root, name))
	if err != nil {
		return ""
	}

	return string(contents)
}

// Has reports whether a path is there, which is most of what a test about generated files asks.
func (p Project) Has(name string) bool {
	_, err := os.Stat(filepath.Join(p.Root, name))

	return err == nil
}
