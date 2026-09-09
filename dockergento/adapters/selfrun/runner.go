// Package selfrun runs this binary again, for the install door.
//
// `setup`'s install steps are commands of this same tool — `start`, `install`, `set-host` and the
// rest — so the door that runs them is this executable, not the shell CLI directly. What is not
// ported yet still reaches the shell tree, one hop later, through the ordinary router and its own
// bridge (`dockergento.Engine.Shell`).
package selfrun

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// resolve answers what runs, in a fail-safe order:
//
//  1. Executable, if set (lets a test or a caller pin exactly what runs)
//  2. HM_SELF, if set
//  3. $HM_LEGACY_ROOT/bin/run, only when HM_LEGACY_ROOT is set — deliberately third, and not
//     first, so that redirecting the shell tree and redirecting this binary stay two separate
//     questions. A suite that redirects the tree without redirecting the binary must not silently
//     fall through to the real os.Executable().
//  4. os.Executable(), as the last resort.
//
// A resolved path that is not executable is an error, not an exec attempt — the same refusal
// `hm_run_binary` makes on the shell side.
func (r Runner) resolve() (string, error) {
	executable := r.Executable

	if executable == "" {
		executable = os.Getenv("HM_SELF")
	}

	if executable == "" {
		if root := os.Getenv("HM_LEGACY_ROOT"); root != "" {
			executable = filepath.Join(root, "bin", "run")
		}
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
