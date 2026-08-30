// Package ports declares what the domain needs from the outside world.
//
// Every one of these is implemented by an adapter and by a fake in the tests. That is the whole
// point of the arrangement: resolving a project, reconciling an environment or collecting what is
// abandoned can be exercised without a Docker daemon, a git repository or a filesystem — which is
// exactly the part of the tool that could not be tested before.
package ports

import "github.com/hiberus-magento/hiberus-dockergento/internal/core"

// Properties reads the configuration a project keeps in config/docker/properties.json.
type Properties interface {
	// Load returns the properties of the project rooted at dir. A project with no properties
	// file is not an error: it is a directory that is not a project, and the caller decides
	// what that means.
	Load(dir string) (map[string]string, error)
}

// VCS answers the questions this tool asks git.
type VCS interface {
	// Resolve reports the main checkout of the repository containing dir, and whether dir is a
	// linked worktree of it. A directory outside a repository is not an error either.
	Resolve(dir string) (mainRoot string, isWorktree bool, worktreeName string, err error)
}

// Registry is where branch environments are recorded, outside the checkout: properties.json is a
// committed file, and a worktree's project name written there would travel in somebody's commit.
type Registry interface {
	// Worktree returns the registration of a branch environment, or nil when there is none.
	// A worktree with no registration is the case the guardrails refuse.
	Worktree(parent, name string) (*core.Worktree, error)
}

// Legacy runs a command of the shell implementation.
//
// It exists because the migration is a strangler and not a rewrite: what has not been ported yet
// still runs, and a project can add commands of its own in config/hm/commands, which will always
// be shell. This port does not go away when the migration ends.
type Legacy interface {
	// Run executes the shell CLI with these arguments, wired to the current terminal, and
	// returns its exit code.
	Run(args []string) (int, error)
}
