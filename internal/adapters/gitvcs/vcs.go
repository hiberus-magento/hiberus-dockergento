// Package gitvcs answers the questions this tool asks git.
package gitvcs

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// Git shells out to the git binary.
//
// Deliberately, and it is the one place in this tool that will keep doing so: go-git's Worktree
// is not a git worktree — linked worktrees are not part of its model — so a library that looked
// like the right dependency would answer a different question.
type Git struct{}

// Resolve reports the main checkout of the repository containing dir, whether dir is a linked
// worktree of it, and the worktree's directory name.
//
// One git call rather than three: asked separately, "am I in a repository", "where is the common
// .git" and "what is this checkout's root" cost about 130 ms, and asked together about 45 ms, on
// a command that runs dozens of times a day.
func (Git) Resolve(dir string) (mainRoot string, isWorktree bool, worktreeName string, err error) {
	output, err := run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel")
	if err != nil {
		// Older git has no --path-format; and outside a repository the call simply fails, which
		// is the same answer the question used to give.
		output, err = run(dir, "rev-parse", "--git-common-dir", "--show-toplevel")
		if err != nil {
			return dir, false, "", nil
		}
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return dir, false, "", nil
	}

	commonDir, toplevel := lines[0], lines[1]

	if !filepath.IsAbs(commonDir) {
		absolute, absErr := filepath.Abs(filepath.Join(dir, commonDir))
		if absErr != nil {
			return dir, false, "", nil
		}
		commonDir = absolute
	}

	root := strings.TrimSuffix(commonDir, string(filepath.Separator)+".git")

	// A bare repository, an unexpected layout, or the main checkout itself
	if root == commonDir || root == toplevel {
		return toplevel, false, "", nil
	}

	return root, true, filepath.Base(toplevel), nil
}

func run(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir

	output, err := command.Output()

	return string(output), err
}
