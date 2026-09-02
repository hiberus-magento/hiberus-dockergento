package app

import (
	"fmt"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Branch environments: a git worktree with an environment of its own.
//
// A worktree with no registration keeps the guardrails that stop it from recreating the main
// checkout's environment with its own mounts. One with a registration has its own containers, its
// own database and its own address, and the difference between the two is a file outside the
// checkout — because config/docker/properties.json is committed, and a worktree's project name
// written there would travel in somebody's commit.
type Worktrees struct {
	Registry     ports.Registry
	Engine       ports.ContainerEngine
	VCS          ports.VCS
	FS           ports.FS
	Orchestrator ports.Orchestrator

	Announce func(string)
	Ask      func(question, suggestion string) (string, error)

	Binary string
}

// Listed is a branch environment as somebody reads it.
type Listed struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Profile string `json:"profile"`
	State   string `json:"state"`
	URL     string `json:"url"`
	Path    string `json:"path"`
}

// List is every branch environment of a project, with what is actually running.
//
// The state comes from the containers and the directory, not from the registration: a registry
// that answered "running" for something somebody stopped by hand would be worse than not asking.
func (w Worktrees) List(parent string) ([]Listed, error) {
	registered, err := w.Registry.Worktrees(parent)
	if err != nil {
		return nil, err
	}

	containers, err := w.Engine.Containers()
	if err != nil {
		return nil, err
	}

	running := map[string]bool{}

	for _, container := range containers {
		if container.Running {
			running[container.ComposeProject] = true
		}
	}

	listed := make([]Listed, 0, len(registered))

	for _, worktree := range registered {
		state := "stopped"

		if running[worktree.Project] {
			state = "running"
		}

		// A directory somebody deleted by hand leaves a registration behind, and saying "stopped"
		// about it would send them looking for containers that are not the problem
		if !w.FS.IsDir(worktree.Path) {
			state = "missing"
		}

		listed = append(listed, Listed{
			Name: worktree.Name, Branch: worktree.Branch, Profile: worktree.Profile,
			State: state, URL: "https://" + worktree.Domain, Path: worktree.Path,
		})
	}

	return listed, nil
}

// Remove takes a branch environment away: its containers, its data, its worktree and its
// registration.
func (w Worktrees) Remove(parent, root, name string, files core.ComposeFiles,
	force, interactive bool) (string, error) {
	if name == "" {
		return "", core.Refusal{
			Kind: "missing_name", Code: 2,
			Message: "Which branch environment should be removed?",
			Hint:    w.Binary + " worktree list",
		}
	}

	worktree, err := w.Registry.Worktree(parent, name)
	if err != nil {
		return "", err
	}

	if worktree == nil {
		return "", core.Refusal{
			Kind: "unknown_worktree", Code: 2,
			Message: fmt.Sprintf("'%s' is not a branch environment of this project", name),
			Hint:    w.Binary + " worktree list",
		}
	}

	//
	// The containers and the database can be rebuilt in seconds; uncommitted code cannot be
	// rebuilt at all, which is why git's own refusal is repeated here rather than worked around.
	//
	if !force && w.FS.IsDir(worktree.Path) && w.VCS.Dirty(worktree.Path) {
		return "", core.Refusal{
			Kind: "uncommitted_changes", Code: 6,
			Message: fmt.Sprintf("%s has uncommitted changes", name),
			Hint:    "Commit them, or repeat with --force",
		}
	}

	if interactive && !force {
		w.announce(fmt.Sprintf(
			"This destroys the environment %s and removes %s.\n\n", worktree.Project, worktree.Path))

		answer, err := w.Ask("Type the name to confirm", "")
		if err != nil || strings.TrimSpace(answer) != name {
			return "", nil
		}
	}

	w.announce(fmt.Sprintf("Removing %s...\n", name))

	//
	// The environment first, with its volumes: the database it was given was a copy, and leaving
	// it behind is how a machine fills up with the data of branches nobody works on any more.
	//
	if w.FS.IsDir(worktree.Path) {
		environment := core.Project{Name: worktree.Project, Root: worktree.Path}

		w.Orchestrator.Down(environment, files, true) //nolint:errcheck
	}

	w.VCS.RemoveWorktree(root, worktree.Path, force) //nolint:errcheck

	// A worktree whose directory a person deleted by hand leaves a stale administrative entry,
	// and git refuses to reuse the name until it is pruned
	w.VCS.Prune(root) //nolint:errcheck

	if err := w.Registry.Forget(parent, name); err != nil {
		return "", err
	}

	return name, nil
}

func (w Worktrees) announce(message string) {
	if w.Announce != nil {
		w.Announce(message)
	}
}
