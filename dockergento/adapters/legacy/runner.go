// Package legacy runs the shell implementation.
//
// This is the strangler: the Go binary is the entry point and everything not ported yet still
// runs, unchanged, through the shell CLI. It is not scaffolding to be removed — a project can add
// commands of its own under config/hm/commands, and those will always be shell.
package legacy

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when the shell implementation cannot be located.
var ErrNotFound = errors.New("the shell implementation was not found next to this binary")

// Runner executes the shell CLI.
type Runner struct {
	// Root is the directory holding bin/run and console/. Empty means: work it out.
	Root string

	// Registration is what the registry says about the branch environment this is being run
	// from, as environment entries, or nothing when it is not being run from one.
	//
	// It is handed over rather than looked up again because the registry is a database now and
	// the shell implementation cannot read it. Without this, a bridged command run inside a
	// branch environment would find no registration and fall back to the main environment —
	// which is the case WT-01 exists to refuse: the main environment's mounts repointed at
	// somebody else's checkout, and its database dropped by a `setup:upgrade` meant for a branch.
	Registration []string
}

// Run executes the shell CLI with these arguments, wired to this process's terminal, and returns
// its exit code.
//
// Standard input, output and error are passed through untouched. That is what makes the
// substitution invisible: an interactive prompt still reads from the terminal, a pipe is still a
// pipe, and the JSON contract still writes the document to stdout and the errors to stderr.
func (r Runner) Run(args []string) (int, error) {
	root, err := r.locate()
	if err != nil {
		return 0, err
	}

	command := exec.Command(filepath.Join(root, "bin", "run"), args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = append(withoutRegistration(os.Environ()), r.Registration...)

	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}

		return 0, err
	}

	return 0, nil
}

// withoutRegistration drops any registration inherited from an outer invocation.
//
// A bridged command can run the tool again from another directory — the vendor dance does exactly
// that, from the main checkout — and two entries for the same name in one environment is a
// question of whose reading wins. Removing the old one leaves no question.
func withoutRegistration(environment []string) []string {
	kept := make([]string, 0, len(environment))

	for _, entry := range environment {
		if strings.HasPrefix(entry, "HM_REGISTERED=") || strings.HasPrefix(entry, "HM_REGISTERED_") {
			continue
		}

		kept = append(kept, entry)
	}

	return kept
}

// locate finds the shell tree.
//
// The override comes first so that a test, or somebody debugging, can point the binary at another
// checkout. Otherwise it walks up from the executable, which covers both the binary living in the
// repository's bin/ and an installation that keeps the tree beside it.
func (r Runner) locate() (string, error) {
	if r.Root != "" {
		return r.Root, nil
	}

	if root := os.Getenv("HM_LEGACY_ROOT"); root != "" {
		return root, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	directory := filepath.Dir(executable)
	for i := 0; i < 4; i++ {
		if isShellTree(directory) {
			return directory, nil
		}
		directory = filepath.Dir(directory)
	}

	return "", ErrNotFound
}

func isShellTree(directory string) bool {
	if _, err := os.Stat(filepath.Join(directory, "bin", "run")); err != nil {
		return false
	}

	_, err := os.Stat(filepath.Join(directory, "console"))

	return err == nil
}
