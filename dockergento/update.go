package dockergento

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Update pulls this tool's own installation, a git checkout, and regenerates the shell completion
// from what was pulled.
//
// An installation checked out at a tag is a version somebody chose, so there is nothing to update
// it to: that is said, with the command that moves it, rather than pulled onto a branch behind
// their back. Ported from update.sh, messages included.
func (e *Engine) Update() error {
	root := e.options.Root

	run := func(args ...string) (string, error) {
		output, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()

		return strings.TrimSpace(string(output)), err
	}

	if _, err := run("rev-parse", "--git-dir"); err != nil {
		return core.Refusal{
			Kind:    "not_a_git_installation",
			Code:    1,
			Message: "This is not a git installation of Hiberus Dockergento",
			Hint:    "Reinstall with the installer to be able to update",
		}
	}

	branch, _ := run("rev-parse", "--abbrev-ref", "HEAD")

	if branch == "HEAD" {
		version, err := run("describe", "--tags", "--abbrev=0")
		if err != nil || version == "" {
			version = "unknown"
		}

		return core.Refusal{
			Kind:    "detached_installation",
			Code:    6,
			Message: fmt.Sprintf("The installation is pinned to %s, so there is nothing to update", version),
			Hint:    e.options.Binary + " switch --stable   # or switch to another version",
		}
	}

	if _, err := run("pull", "origin", branch); err != nil {
		return core.Refusal{
			Kind:    "update_failed",
			Code:    1,
			Message: "Could not update from origin/" + branch,
			Hint:    "Check the installation directory: " + root,
		}
	}

	if err := exec.Command(filepath.Join(root, "generate_completion.sh")).Run(); err != nil {
		return core.Refusal{
			Kind:    "completion_failed",
			Code:    1,
			Message: "Hiberus Dockergento was updated, but its shell completion could not be generated: " + err.Error(),
			Hint:    "Run " + filepath.Join(root, "generate_completion.sh") + " to see why",
		}
	}

	return nil
}
