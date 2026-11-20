package machine

import (
	"os"
	"os/exec"
)

// Programs runs programs on this machine, attached to a terminal.
//
// The streams are the terminal's: what is installed asks for a password through sudo, and brew
// and apt say what they are doing as they do it. Nil is this process's own.
//
// Quiet runs them on the null device instead, for a program whose chatter is nobody's business:
// mkcert signing the proxy's certificates in the middle of setup, as the shell's >/dev/null did.
type Programs struct {
	Stdin, Stdout, Stderr *os.File

	Quiet bool
}

// LookPath is where a program is on the PATH.
func (Programs) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Run executes a program in dir and fails when it does not exit 0.
func (p Programs) Run(dir, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = dir

	// Left unset, exec connects all three to the null device
	if p.Quiet {
		return command.Run()
	}

	command.Stdin = orOwn(p.Stdin, os.Stdin)
	command.Stdout = orOwn(p.Stdout, os.Stdout)
	command.Stderr = orOwn(p.Stderr, os.Stderr)

	return command.Run()
}

// orOwn is a stream, or this process's own when none was given. A nil *os.File handed to exec
// as an interface would not read as nil.
func orOwn(stream, own *os.File) *os.File {
	if stream == nil {
		return own
	}

	return stream
}
