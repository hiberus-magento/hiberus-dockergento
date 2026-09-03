package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

//
// The parts that need a daemon.
//
// A test that cannot run is better than a test that fails, so these skip rather than fail where
// there is no Docker — which is also what makes the package usable on a machine that only has Go.
//

var (
	dockerOnce      sync.Once
	dockerAvailable bool
)

// NeedsDocker skips the test when it should not run: in short mode, or where there is no daemon to
// talk to.
//
// Two reasons and not one. `go test -short` is how somebody asks for the fast answer, whatever the
// machine has; the daemon check is what makes `go test ./...` usable on a machine that only has Go.
// A test that cannot run is better than a test that fails.
func NeedsDocker(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping: needs Docker, and -short was asked for")
	}

	dockerOnce.Do(func() {
		dockerAvailable = exec.Command("docker", "info").Run() == nil
	})

	if !dockerAvailable {
		t.Skip("skipping: no Docker daemon")
	}
}

// Compose runs a compose command against a test's project, for the scene-setting a test does on
// its own behalf rather than through the tool.
func Compose(t *testing.T, session *Session, project Project, args ...string) string {
	t.Helper()

	arguments := append([]string{"compose", "-p", project.Name}, args...)

	command := exec.Command("docker", arguments...)
	command.Dir = project.Root
	command.Env = append(os.Environ(), session.environment()...)

	output, _ := command.CombinedOutput()

	return strings.TrimSpace(string(output))
}

// Up brings the project's environment up and takes it down when the test ends.
//
// Down with its volumes and its orphans: what a test leaves behind is a container somebody has to
// find later, and the name it leaves it under is one this suite chose.
func Up(t *testing.T, session *Session, project Project) {
	t.Helper()

	Compose(t, session, project, "up", "-d")

	t.Cleanup(func() {
		Compose(t, session, project, "down", "-v", "--remove-orphans")
	})
}

// Inside runs a shell command in the project's php container and answers what it said.
func Inside(t *testing.T, session *Session, project Project, command string) string {
	t.Helper()

	return trimmed(Compose(t, session, project, "exec", "-T", "phpfpm", "sh", "-c", command))
}

// InsideService is the same for a service that is not the php one.
func InsideService(t *testing.T, session *Session, project Project, service, command string) string {
	t.Helper()

	return trimmed(Compose(t, session, project, "exec", "-T", service, "sh", "-c", command))
}

// Ready waits until a command inside a service succeeds, which is how a database is waited for.
//
// A container that is running is not the same thing as a database that is answering, and the
// difference is the first thirty seconds of its life.
func Ready(t *testing.T, session *Session, project Project, service, check string) bool {
	t.Helper()

	deadline := time.Now().Add(90 * time.Second)

	for time.Now().Before(deadline) {
		arguments := []string{"compose", "-p", project.Name, "exec", "-T", service, "sh", "-c", check}

		command := exec.Command("docker", arguments...)
		command.Dir = project.Root
		command.Env = append(os.Environ(), session.environment()...)
		command.Stdout, command.Stderr = &bytes.Buffer{}, &bytes.Buffer{}

		if command.Run() == nil {
			return true
		}

		time.Sleep(2 * time.Second)
	}

	return false
}

func trimmed(output string) string { return strings.TrimSpace(output) }
