// Package core holds the domain: what a project, a worktree and an environment are, with no
// knowledge of Docker, git, the filesystem or the terminal.
//
// Nothing in this package imports an adapter, and nothing in it prints. That is not tidiness for
// its own sake: it is what lets the same logic serve the CLI, the MCP server and — later — an
// HTTP API, and what lets the reconciliation be tested in milliseconds without a Docker daemon.
package core

// Topology is how a project's services are arranged.
//
// Both are first class. Most of the department's projects cannot fit the orchestrated model — a
// different Magento version, a different search engine, a stack of their own — and a tool that
// only knew how to orchestrate would be a tool nobody could migrate to.
type Topology string

const (
	// Classic is one stack per project: what every project does today.
	Classic Topology = "classic"

	// Orchestrated is shared state services with logical isolation, and only the application
	// runtime per worktree.
	Orchestrated Topology = "orchestrated"
)

// Project is a Magento project as this tool sees it.
type Project struct {
	// Name is the compose project name: what decides which containers, which volumes and which
	// database are used. It is never generated when it collides — a name nobody chose is how
	// somebody ends up working in an environment they did not mean to open.
	Name string

	// Root is the directory the configuration is resolved against. For a registered worktree it
	// is the worktree, not the main checkout.
	Root string

	// Domain is the address the environment answers on.
	Domain string

	// MagentoDir is where Magento lives inside Root, usually ".".
	MagentoDir string

	Topology Topology

	// Worktree is set when Root is a git worktree with an environment of its own. It is nil for
	// a main checkout, and for a worktree that has no environment — which is a different thing,
	// and the difference is what keeps the second from destroying the first.
	Worktree *Worktree
}

// Worktree is a branch environment: a second working directory of the same repository with its
// own containers, address and data.
type Worktree struct {
	// Name is the slug the environment is known by, and the first label of its address.
	Name string

	// Project and Domain are read from the registration rather than derived from the parent's.
	// Deriving them would be a second place where the naming rule lives, and the day the two
	// disagreed the environment would answer to a name nothing created.
	Project string
	Domain  string

	// Path is the worktree's own directory, which is what its relative bind mounts resolve
	// against, and ParentRoot is the main checkout it was made from.
	Path       string
	ParentRoot string

	// Branch is the git branch checked out in it.
	Branch string

	// Parent is the compose project name of the main checkout.
	Parent string

	// Profile decides which services run: lite, agent or full.
	Profile string

	// SharedVendor says whether the dependencies are read from the main checkout. They are
	// mounted, never linked: PHP resolves __DIR__ through a symlink, and with one the worktree's
	// own modules are never registered.
	SharedVendor bool
}

// IsWorktree reports whether this project is a branch environment with its own containers.
func (p Project) IsWorktree() bool {
	return p.Worktree != nil
}
