package core

//
// What abandoned environments leave behind.
//
// The whole design of this is about what it is not allowed to touch. `docker system prune` already
// exists and is the wrong tool: it cannot tell this tool's leftovers from somebody's hand-written
// stack, or a dead project from a stopped one.
//

// Collection is everything found, and whether it was removed.
type Collection struct {
	Removed bool `json:"removed"`

	// Environments are ours and their directory is gone. A stopped project whose directory is
	// still there is not rubbish, it is a stopped project — and that distinction is the reason
	// this exists.
	Environments []Abandoned `json:"environments"`

	// Volumes belong to those environments, or are frozen data directories whose project is gone.
	Volumes []string `json:"volumes"`

	// Worktrees are registrations whose worktree somebody removed with git or deleted by hand.
	// Nothing else deletes them: `worktree remove` is the tidy path and it needs the directory.
	Worktrees []AbandonedWorktree `json:"worktrees"`

	// Hosts are entries this tool put in /etc/hosts with no environment left. Listed and never
	// removed: it needs a password, other things depend on that file, and a command that quietly
	// rewrites it in the middle of an unrelated cleanup is one nobody trusts twice.
	Hosts []string `json:"hosts"`

	// Unattributable is what cannot be judged from here, and is therefore left alone.
	Unattributable struct {
		Volumes      []string    `json:"volumes"`
		Environments []Abandoned `json:"environments"`
	} `json:"unattributable"`
}

// Abandoned is an environment with nothing left to belong to.
type Abandoned struct {
	Name string `json:"name"`

	// Root is where it was, and Reason is why it cannot be judged when there is no root to check.
	Root   string `json:"root,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// AbandonedWorktree is a registration whose worktree is gone.
type AbandonedWorktree struct {
	Name    string `json:"name"`
	Project string `json:"project"`
	Path    string `json:"path"`
}

// Anything reports whether there is something to collect at all.
func (c Collection) Anything() bool {
	return len(c.Environments) > 0 || len(c.Worktrees) > 0
}
