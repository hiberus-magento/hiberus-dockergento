// Package selfrun runs this binary again, for the install door and for the MCP server's tools.
//
// `setup`'s install steps are commands of this same tool — `start`, `install`, `set-host` and the
// rest — so the door that runs them is this executable.
package selfrun

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
)

// Runner runs this binary again with a step's arguments, wired to this process's terminal, and
// answers with its exit code.
type Runner struct {
	// Executable, if set, is exactly what runs. This is what lets a test — or a caller — pin the
	// binary without touching the environment.
	Executable string
}

// Run executes the resolved executable with these arguments, wired to this process's terminal, and
// returns its exit code.
//
// Standard input, output and error are passed through untouched, which is what keeps a step's own
// prompt — `set-host`'s `sudo`, among others — on the caller's terminal.
func (r Runner) Run(args []string) (int, error) {
	executable, err := r.resolve()
	if err != nil {
		return 0, err
	}

	command := exec.Command(executable, args...)
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

// Capture executes the resolved executable with these arguments and its standard input closed,
// and returns what it printed: standard output, and standard error with it when combined —
// otherwise standard error is dropped. An exit code is not an error: the output says how it went.
//
// This is the MCP server's door, where each tool is a command of this same binary whose answer
// becomes a protocol message, and nothing may reach the terminal or read the conversation.
func (r Runner) Capture(args []string, combined bool) ([]byte, error) {
	output, _, err := r.CaptureStatus(args, combined)

	return output, err
}

// CaptureStatus is Capture with the exit code it ended with.
func (r Runner) CaptureStatus(args []string, combined bool) ([]byte, int, error) {
	executable, err := r.resolve()
	if err != nil {
		return nil, 0, err
	}

	var output bytes.Buffer

	command := exec.Command(executable, args...)
	command.Stdout = &output
	if combined {
		command.Stderr = &output
	}

	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return output.Bytes(), 0, err
		}

		return output.Bytes(), exit.ExitCode(), nil
	}

	return output.Bytes(), 0, nil
}

// resolve answers what runs, in a fail-safe order:
//
//  1. Executable, if set (lets a test or a caller pin exactly what runs)
//  2. HM_SELF, if set
//  3. os.Executable(), as the last resort.
//
// A resolved path that is not executable is an error, not an exec attempt — the same refusal
// `hm_run_binary` makes on the shell side.
func (r Runner) resolve() (string, error) {
	executable := r.Executable

	if executable == "" {
		executable = os.Getenv("HM_SELF")
	}

	if executable == "" {
		resolved, err := os.Executable()
		if err != nil {
			return "", err
		}

		executable = resolved
	}

	info, err := os.Stat(executable)
	if err != nil {
		return "", err
	}

	if info.Mode()&0o111 == 0 {
		return "", errors.New(executable + " is not executable")
	}

	return executable, nil
}
