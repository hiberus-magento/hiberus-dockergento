package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hmRoot is where the project's own configuration is looked for: its commands, their descriptions
// and its properties. Ported from console/helpers/worktree.sh's hm_resolve_project_root, which
// bin/run resolved before anything else.
//
// The rule is the shell's to the letter, and so is what it leaves out. HM_PROJECT_DIR wins. From a
// worktree of a Dockergento project the root is the main checkout. Anywhere else — the main
// checkout's own root, one of its subdirectories, a directory outside any repository — it is the
// directory the command was run in, not the top of the repository: a project command found from
// the root is not found from a subdirectory, and that has always been so.
type hmRoot struct {
	Root       string
	IsWorktree bool
	Worktree   string
}

func resolveHMRoot(pwd string) hmRoot {
	if dir := os.Getenv("HM_PROJECT_DIR"); dir != "" {
		return hmRoot{Root: dir}
	}

	here := hmRoot{Root: pwd}

	output, err := gitIn(pwd, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel")
	if err != nil {
		if output, err = gitIn(pwd, "rev-parse", "--git-common-dir", "--show-toplevel"); err != nil {
			return here
		}
	}

	lines := strings.SplitN(output, "\n", 3)
	if len(lines) < 2 || lines[0] == "" || lines[1] == "" {
		return here
	}

	common, toplevel := lines[0], lines[1]

	if !filepath.IsAbs(common) {
		common = filepath.Join(pwd, common)
	}

	main := strings.TrimSuffix(common, "/.git")
	if main == common || main == toplevel {
		return here
	}

	for _, required := range []string{"docker-compose.yml", filepath.Join("config", "docker", "properties.json")} {
		if info, err := os.Stat(filepath.Join(main, required)); err != nil || info.IsDir() {
			return here
		}
	}

	return hmRoot{Root: main, IsWorktree: true, Worktree: filepath.Base(toplevel)}
}

func gitIn(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir

	output, err := command.Output()

	return string(output), err
}

// customCommandsDir is where a project keeps commands of its own: config/hm/commands, with the
// name bin/run hardcoded rather than the name the binary was invoked as.
func customCommandsDir(root string) string {
	return filepath.Join(root, "config", "hm", "commands")
}
