package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The service everything PHP runs in. Named once: three commands mean the same container by it.
const phpService = "phpfpm"

// magento is the Magento CLI, which is the command people run most of all.
func magento(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := codeProjectOr(stderr, jsonOutput, "magento")
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

	project, code := codeProjectOr(stderr, jsonOutput, "composer")
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

	if mirrorsVendor(args) {
		return composerMirroringVendor(project, args, stdout, stderr, jsonOutput)
	}

	return inside(project, append([]string{"composer"}, args...), "",
		stdout, stderr, jsonOutput, "composer")
}

// composerMirroringVendor is Composer on macOS for the four subcommands that write the
// dependencies, where the code lives in a volume rather than a mount: the host's vendor is copied
// in, Composer runs, and the whole workdir is copied back out over the host's.
//
// Ported from composer.sh, step for step and in its order. A step that fails stops the rest with
// its own code, as set -e did there: copying back a tree Composer never finished writing, or
// clearing the host's vendor when nothing will replace it, is how a failed install becomes a lost
// one.
func composerMirroringVendor(project core.Project, args []string,
	stdout, stderr io.Writer, jsonOutput bool) int {
	//
	// From a worktree with no environment of its own the dance would restart phpfpm and rewrite
	// the dependencies of the main checkout.
	//
	// The engine refuses the restart anyway, but its way out says "repeat with --force", and after
	// `composer` that flag is Composer's: `hm composer install --force` never reaches hm. So it is
	// said here, before the restart, with the form hm does read.
	//
	if project.ServesAnotherCheckout() && os.Getenv("HM_FORCE") != "1" {
		return failure(stderr, jsonOutput, "composer", exitBlocked, "blocked_in_worktree",
			fmt.Sprintf("'composer %s' is refused from a worktree on macOS: it would restart phpfpm and rewrite the dependencies of %s",
				args[0], project.Root),
			fmt.Sprintf("Run it from %s, or repeat as %s --force composer %s",
				project.Root, binaryName(), strings.Join(args, " ")))
	}

	tool := newEngine(stdout, stderr, jsonOutput)
	dir := here()
	fail := func(err error) int { return report(stderr, jsonOutput, "composer", err) }

	if err := tool.Restart(dir, core.RestartOptions{Services: []string{phpService}}); err != nil {
		return fail(err)
	}

	if err := tool.CopyInto(dir, []string{"vendor"}, false); err != nil {
		return fail(err)
	}

	if code := inside(project, append([]string{"composer"}, args...), "",
		stdout, stderr, jsonOutput, "composer"); code != 0 {
		return code
	}

	// The files a Magento project ships at its root go in too, so that the copy back out, which
	// covers the whole workdir, brings back the host's own and not whatever Composer left there
	files, err := tool.DefaultMagentoFiles()
	if err != nil {
		return fail(err)
	}

	if err := tool.CopyInto(dir, files, false); err != nil {
		return fail(err)
	}

	// Stopped for the copy back out, so that nothing writes to the tree while it is being read
	if err := tool.Stop(dir, core.StopOptions{Services: []string{phpService}}); err != nil {
		return fail(err)
	}

	if err := tool.ClearVendor(dir); err != nil {
		return fail(err)
	}

	if err := tool.CopyFrom(dir, []string{"."}); err != nil {
		return fail(err)
	}

	return fail(tool.Start(dir, dockergento.StartOptions{Services: []string{phpService}}))
}

// inside runs a command in the php container and answers with that command's own exit code.
func inside(project core.Project, command []string, user string,
	stdout, stderr io.Writer, jsonOutput bool, name string) int {
	status, err := newEngine(stdout, stderr, jsonOutput).
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
// dependency tree: one of the four subcommands that write the dependencies, on macOS.
//
// There they do not simply run in the container: the dependencies are copied in, Composer runs,
// and the whole tree is copied back out over the host's — which empties the host's vendor
// directory on the way. On Linux the code is a bind mount and the same four just run.
func mirrorsVendor(args []string) bool {
	return goos == "darwin" && writesDependencies(args)
}

// goos is the platform the vendor dance is decided on. A variable so that a test on Linux can
// pin both answers rather than skip one of them; production never sets it.
var goos = runtime.GOOS
