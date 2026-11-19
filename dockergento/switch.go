package dockergento

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Moving this tool's own installation, a git checkout, between versions and branches. Ported from
// switch.sh, messages included, and with its git commands: a tag stays detached, which `hm update`
// refuses on purpose, and a branch of origin's becomes a local branch tracking it, so that `hm
// update` keeps working there.
//

// looksLikeVersion is the grep switch.sh filtered tags with: anything starting like a version.
var looksLikeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`)

// installationGit runs git in the installation. What comes back loses its trailing newlines, the
// way a shell's $(...) loses them, and nothing else: the leading space of a porcelain line means
// something.
type installationGit string

func (g installationGit) run(args ...string) (string, error) {
	output, err := exec.Command("git", append([]string{"-C", string(g)}, args...)...).Output()

	return strings.TrimRight(string(output), "\n"), err
}

// combined is run with both streams together, which is how switch.sh kept what a failed checkout
// said.
func (g installationGit) combined(args ...string) (string, error) {
	output, err := exec.Command("git", append([]string{"-C", string(g)}, args...)...).CombinedOutput()

	return strings.TrimRight(string(output), "\n"), err
}

func (g installationGit) verifies(reference string) bool {
	_, err := g.run("rev-parse", "--verify", "--quiet", reference)

	return err == nil
}

// requireCheckout is the installer's own layout, and what makes switching a git operation at all.
func (g installationGit) requireCheckout() error {
	if _, err := g.run("rev-parse", "--git-dir"); err != nil {
		return core.Refusal{
			Kind:    "not_a_git_installation",
			Code:    1,
			Message: "This is not a git installation of Hiberus Dockergento",
			Hint:    "Reinstall with the installer to be able to switch versions",
		}
	}

	return nil
}

// refresh fetches, because without it a newly published version is invisible. Offline is not a
// reason to refuse: what is already known can still be switched to.
func (g installationGit) refresh() {
	_, _ = g.run("fetch", "--tags", "--prune", "origin")
}

// current is the branch checked out, or the nearest tag of a detached checkout.
func (g installationGit) current() string {
	branch, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		branch = ""
	}

	if branch != "HEAD" {
		return branch
	}

	tag, err := g.run("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "unknown"
	}

	return tag
}

func lines(text string) []string {
	found := []string{}

	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			found = append(found, line)
		}
	}

	return found
}

// Versions is what the installation can be switched to: the tags that look like versions, newest
// first, and origin's branches.
func (e *Engine) Versions() (core.Versions, error) {
	git := installationGit(e.options.Root)

	if err := git.requireCheckout(); err != nil {
		return core.Versions{}, err
	}

	git.refresh()

	current := git.current()
	versions := []core.Version{}

	tags, _ := git.run("tag", "--list", "--sort=-version:refname")
	for _, tag := range lines(tags) {
		if !looksLikeVersion.MatchString(tag) {
			continue
		}

		versions = append(versions, core.Version{
			Name:       tag,
			PreRelease: strings.Contains(tag, "-"),
			Installed:  tag == current,
		})
	}

	branches := []string{}

	references, _ := git.run("for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	for _, reference := range lines(references) {
		branch := strings.TrimPrefix(reference, "origin/")
		if branch != "HEAD" {
			branches = append(branches, branch)
		}
	}

	return core.Versions{Current: current, Versions: versions, Branches: branches}, nil
}

// Switch moves the installation to a tag or a branch, and regenerates the shell completion from
// what is there now.
//
// Never over somebody's work: tracked changes refuse the switch, with what they are. Untracked
// files do not — notes, editor configuration, an agent's skills are always there, and git refuses
// by itself when the target would overwrite one.
func (e *Engine) Switch(reference string) (core.Switched, error) {
	root := e.options.Root
	git := installationGit(root)

	if err := git.requireCheckout(); err != nil {
		return core.Switched{}, err
	}

	if changes, _ := git.run("status", "--porcelain", "--untracked-files=no"); changes != "" {
		return core.Switched{}, core.DirtyInstallation{Directory: root, Changes: changes}
	}

	git.refresh()

	if !git.verifies(reference) && !git.verifies("origin/"+reference) {
		return core.Switched{}, core.Refusal{
			Kind:    "unknown_reference",
			Code:    2,
			Message: fmt.Sprintf("There is no version or branch called '%s'", reference),
			Hint:    e.options.Binary + " switch --list",
		}
	}

	var failure string

	switch {
	case git.verifies("refs/tags/" + reference), git.verifies("refs/heads/" + reference):
		failure, _ = git.combined("checkout", "--quiet", reference)
	default:
		failure, _ = git.combined("checkout", "--quiet", "-B", reference, "--track", "origin/"+reference)
	}

	now := git.current()

	if now != reference && failure != "" {
		return core.Switched{}, core.Refusal{
			Kind:    "switch_failed",
			Code:    1,
			Message: fmt.Sprintf("Could not switch to '%s': %s", reference, failure),
			Hint:    "Check the installation directory: " + root,
		}
	}

	// As the shell did: the switch has happened, and a completion that could not be regenerated
	// is not a reason to report that it has not
	_ = exec.Command(filepath.Join(root, "generate_completion.sh")).Run()

	description, err := git.run("describe", "--tags")
	if err != nil {
		description = now
	}

	return core.Switched{Reference: reference, Version: description}, nil
}
