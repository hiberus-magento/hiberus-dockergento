// Package app holds the use cases: what the tool does, written against the ports rather than
// against Docker, git or the filesystem.
package app

import (
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Resolver answers the first question every command asks: what project am I in?
type Resolver struct {
	Properties ports.Properties
	VCS        ports.VCS
	Registry   ports.Registry
}

// Resolve reports the project the directory belongs to.
//
// The order matters and is the fix for a defect the shell version carried: the properties are
// read from the root that ends up being used. Reading them from the main checkout and then
// deciding the root was a worktree meant a worktree read — and saved — somebody else's
// configuration.
//
// A directory that is not a project is not an error. Several commands run outside one, and
// turning "there is nothing here" into a failure at this level would take that decision away
// from them.
func (r Resolver) Resolve(dir string) (core.Project, error) {
	root := dir

	mainRoot, isWorktree, worktreeName, err := r.VCS.Resolve(dir)
	if err != nil {
		return core.Project{}, err
	}

	// From a worktree everything is resolved against the main checkout, unless the worktree has
	// an environment of its own. Telling those two apart is what keeps the second from
	// recreating the first with its own mounts.
	if isWorktree {
		root = mainRoot
	}

	properties, err := r.Properties.Load(root)
	if err != nil {
		return core.Project{}, err
	}

	project := projectFrom(root, properties)

	if !isWorktree {
		return project, nil
	}

	worktree, err := r.Registry.Worktree(project.Name, worktreeName)
	if err != nil {
		return core.Project{}, err
	}

	if worktree == nil {
		// A worktree with no environment: it keeps resolving against the main checkout, which
		// is what the refusals depend on.
		return project, nil
	}

	//
	// With one, it resolves against itself — its own root, its own name, its own properties.
	//
	// The name and the address come from the registration and are not derived from the parent's:
	// the worktree's properties.json is the same committed file as the main checkout's and names
	// the same compose project, so reading it back would undo exactly the identity this
	// environment is built on.
	//
	registered := projectFrom(dir, mustLoad(r.Properties, dir, properties))
	registered.Name = worktree.Project
	registered.Domain = worktree.Domain
	registered.Worktree = worktree
	registered.Worktree.Parent = project.Name
	registered.Worktree.ParentRoot = mainRoot

	if registered.Name == "" {
		registered.Name = project.Name + "-" + worktree.Name
	}

	if registered.Domain == "" {
		registered.Domain = worktree.Name + "." + project.Domain
	}

	return registered, nil
}

// mustLoad falls back to what was already read rather than failing: a worktree whose properties
// cannot be read is a worktree of the same commit, so the main checkout's are the right answer
// and the alternative is refusing to work at all.
func mustLoad(properties ports.Properties, dir string, fallback map[string]string) map[string]string {
	loaded, err := properties.Load(dir)
	if err != nil || len(loaded) == 0 {
		return fallback
	}

	return loaded
}

func projectFrom(root string, properties map[string]string) core.Project {
	topology := core.Classic
	if properties["TOPOLOGY"] == string(core.Orchestrated) {
		topology = core.Orchestrated
	}

	magentoDir := properties["MAGENTO_DIR"]
	if magentoDir == "" {
		magentoDir = "."
	}

	return core.Project{
		Name:       properties["COMPOSE_PROJECT_NAME"],
		Root:       root,
		Domain:     properties["DOMAIN"],
		MagentoDir: magentoDir,
		Topology:   topology,
	}
}
