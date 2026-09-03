package registry

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The registry as the live source.
//
// What the shell implementation wrote is drained rather than abandoned: the JSON files are read on
// the way in and removed when their worktree is forgotten, so a machine with branch environments
// in it keeps them, and nothing is left behind claiming to be true.
//
// The overlay stays a file. It is a compose file that Docker reads, and a compose file in a
// database is a compose file nothing can load.
//

// Legacy is where the shell implementation kept its registrations.
type Legacy struct {
	Dir string
}

// Registrations is the store with the old directory beside it.
type Registrations struct {
	Store  *Store
	Legacy Legacy
}

// Worktree returns the registration of a branch environment, or nil when there is none.
func (r Registrations) Worktree(parent, name string) (*core.Worktree, error) {
	if err := r.drain(); err != nil {
		return nil, err
	}

	return r.Store.Worktree(parent, name)
}

// Worktrees is every branch environment of a project.
func (r Registrations) Worktrees(parent string) ([]core.Worktree, error) {
	if err := r.drain(); err != nil {
		return nil, err
	}

	return r.Store.Worktrees(parent)
}

// Parents is every project that has registrations.
func (r Registrations) Parents() ([]string, error) {
	if err := r.drain(); err != nil {
		return nil, err
	}

	projects, err := r.Store.Projects()
	if err != nil {
		return nil, err
	}

	parents := make([]string, 0, len(projects))

	for _, project := range projects {
		worktrees, err := r.Store.Worktrees(project.Name)
		if err != nil || len(worktrees) == 0 {
			continue
		}

		parents = append(parents, project.Name)
	}

	sort.Strings(parents)

	return parents, nil
}

// Save records a branch environment.
func (r Registrations) Save(parent string, worktree core.Worktree) error {
	return r.Store.Register(core.Project{Name: parent}, worktree)
}

// Forget removes the registration, the overlay and whatever the old directory still holds for it.
//
// The last of the three is not tidiness: the old file is read on the way in, so leaving it would
// bring the worktree back the next time anything asked.
func (r Registrations) Forget(parent, name string) error {
	if err := r.Store.Forget(parent, name); err != nil {
		return err
	}

	if err := os.Remove(r.Overlay(parent, name)); err != nil && !os.IsNotExist(err) {
		return err
	}

	if r.Legacy.Dir != "" {
		os.Remove(filepath.Join(r.Legacy.Dir, parent, name+".json")) //nolint:errcheck
		os.Remove(filepath.Join(r.Legacy.Dir, parent))               //nolint:errcheck
	}

	return nil
}

// Allocation is the slot a branch environment holds of its project's shared services, when the
// project is orchestrated. Classic projects hold none, and that is not an error.
func (r Registrations) Allocation(parent, name string) (*core.Allocation, error) {
	return r.Store.Allocation(parent, name)
}

// Overlay is the compose file that carries a branch environment's profile and routing.
func (r Registrations) Overlay(parent, name string) string {
	return filepath.Join(r.Legacy.Dir, parent, name+".yml")
}

// WriteOverlay writes it.
func (r Registrations) WriteOverlay(parent, name, contents string) error {
	if err := os.MkdirAll(filepath.Join(r.Legacy.Dir, parent), 0o755); err != nil {
		return err
	}

	return os.WriteFile(r.Overlay(parent, name), []byte(contents), 0o644) //nolint:gosec
}

// drain brings across whatever the old directory still holds. Idempotent, and cheap when there is
// nothing: one directory listing.
func (r Registrations) drain() error {
	if r.Legacy.Dir == "" {
		return nil
	}

	_, err := r.Store.Import(r.Legacy.Dir, "")

	return err
}
