package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The service everything PHP runs in. Named once: three commands mean the same container by it.
const phpService = "phpfpm"

// magento is the Magento CLI, which is the command people run most of all.
func magento(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "magento")
	if code != 0 {
		return code
	}

	return inside(project, append([]string{"php", "./bin/magento"}, args...), "",
		stdout, stderr, jsonOutput, "magento")
}

// composer is Composer, with the two things it is not allowed to do said before it starts.
func composer(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 && args[0] == "create-project" {
		return failure(stderr, jsonOutput, "composer", exitUsage, "wrong_command",
			fmt.Sprintf("create-project is not compatible with %s", binaryName()),
			binaryName()+" create-project")
	}

	project, code := projectOr(stderr, jsonOutput, "composer")
	if code != 0 {
		return code
	}

	//
	// A worktree that reads the main checkout's dependencies cannot write to them.
	//
	// The mount is read-only, so Composer would fail on a read-only filesystem — an error about
	// permissions, three layers down, for something that is a decision rather than an accident.
	//
	if writesDependencies(args) && project.Worktree != nil && project.Worktree.SharedVendor {
		return failure(stderr, jsonOutput, "composer", exitBlocked, "shared_dependencies",
			fmt.Sprintf("This worktree reads the dependencies of %s, so Composer cannot write to them",
				project.Worktree.ParentRoot),
			fmt.Sprintf("Change composer.json in this branch and run %s worktree remove/add, or work on dependencies in the main checkout", binaryName()))
	}

	return inside(project, append([]string{"composer"}, args...), "",
		stdout, stderr, jsonOutput, "composer")
}

// inside runs a command in the php container and answers with that command's own exit code.
func inside(project core.Project, command []string, user string,
	stdout, stderr io.Writer, jsonOutput bool, name string) int {
	status, err := engine(stdout, stderr, jsonOutput).
		Exec(project.Root, phpService, command, terminalOptions(user))
	if status != 0 {
		return status
	}

	return report(stderr, jsonOutput, name, err)
}

// terminalOptions asks for a terminal only when there is one, which is what the docker CLI does:
// asking for one where there is none is how a command that works by hand fails in CI.
func terminalOptions(user string) core.ExecOptions {
	return core.ExecOptions{
		User:        user,
		Interactive: true,
		Tty:         isTerminal(os.Stdin) && isTerminal(os.Stdout),
	}
}

func writesDependencies(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "install", "update", "require", "remove":
		return true
	}

	return false
}

// mirrorsVendor reports whether this is the Composer invocation that rewrites the host's
// dependency tree, which is still the shell implementation's.
//
// On macOS those four subcommands do not simply run in the container: the dependencies are copied
// in, Composer runs, and the whole tree is copied back out over the host's — which deletes the
// host's vendor directory on the way. It depends on `copy-to-container`, which is not ported, and
// it is not a thing to port half of.
func mirrorsVendor(args []string) bool {
	return runtime.GOOS == "darwin" && writesDependencies(args)
}
