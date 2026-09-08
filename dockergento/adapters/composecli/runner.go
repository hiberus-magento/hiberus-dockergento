// Package composecli runs Compose as a subprocess, for the subcommands this tool does not
// implement through composelib.
//
// A deliberate exception, the same one gitvcs makes for git and legacy.Runner makes for bin/run:
// composelib implements operations, not a command line, so a passthrough has to be a subprocess.
// No new dependency — it is os/exec over a binary that is already required.
package composecli

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Runner is the Compose command line itself.
type Runner struct {
	// Command is how Compose is invoked on this machine — "docker compose" or "docker-compose" —
	// split with strings.Fields into an argv and never handed to a shell. Empty means it could
	// not be found on this machine at all.
	Command string
}

// Run executes a Compose subcommand from dir, against these files and this environment, wired to
// this process's own terminal so an interactive subcommand still reads and writes it directly.
func (r Runner) Run(dir string, files []string, environment map[string]string, args []string) (int, error) {
	if r.Command == "" {
		return 0, core.Refusal{
			Kind:    "compose_missing",
			Code:    3,
			Message: "No Compose binary is installed",
			Hint:    "Install Docker or Docker Compose",
		}
	}

	argv := strings.Fields(r.Command)

	for _, file := range files {
		argv = append(argv, "-f", file)
	}

	// Deliberately added where bin/run passed none outside a worktree: relative bind mounts have
	// to resolve against the project, not the caller's own directory — what composelib already
	// does for every ported command.
	argv = append(argv, "--project-directory", dir)
	argv = append(argv, args...)

	command := exec.Command(argv[0], argv[1:]...) //nolint:gosec
	command.Dir = dir
	command.Env = append(os.Environ(), flatten(environment)...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}

		return 0, err
	}

	return 0, nil
}

func flatten(environment map[string]string) []string {
	entries := make([]string, 0, len(environment))

	for key, value := range environment {
		entries = append(entries, key+"="+value)
	}

	return entries
}
