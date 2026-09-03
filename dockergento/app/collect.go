package app

import (
	"sort"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Collecting what abandoned environments left behind.
//
// Looking is the default and deleting is asked for, that way round on purpose: a dry run somebody
// has to remember to type protects the people who were already being careful.
type Collector struct {
	Engine   ports.ContainerEngine
	Volumes  ports.VolumeStore
	Registry ports.Registry
	FS       ports.FS
	Machine  ports.Machine

	// Marker is what this tool writes beside its own entries in /etc/hosts.
	Marker string
}

// Survey finds everything without touching any of it.
func (c Collector) Survey() (core.Collection, error) {
	collection := core.Collection{
		Environments: []core.Abandoned{},
		Volumes:      []string{},
		Worktrees:    []core.AbandonedWorktree{},
		Hosts:        []string{},
	}

	collection.Unattributable.Volumes = []string{}
	collection.Unattributable.Environments = []core.Abandoned{}

	containers, err := c.Engine.Containers()
	if err != nil {
		return collection, err
	}

	//
	// An environment is collectable when both are true: we made it, and its directory is gone.
	//
	// A stopped project whose directory is still there is not rubbish, it is a stopped project.
	//
	roots := map[string]string{}
	labelled := map[string]bool{}
	projects := map[string]bool{}
	unlabelled := map[string]bool{}

	for _, container := range containers {
		projects[container.ComposeProject] = true

		if container.Project == "" {
			// Known only by their php service, with no recorded root to check
			if container.ComposeService == "phpfpm" && container.ComposeProject != "" {
				unlabelled[container.ComposeProject] = true
			}

			continue
		}

		labelled[container.Project] = true

		if _, seen := roots[container.Project]; !seen || container.Root != "" {
			roots[container.Project] = container.Root
		}
	}

	collectable := map[string]bool{}

	for name := range labelled {
		root := roots[name]

		if root == "" {
			collection.Unattributable.Environments = append(collection.Unattributable.Environments,
				core.Abandoned{Name: name, Reason: "no recorded directory"})

			continue
		}

		if c.FS.IsDir(root) {
			continue
		}

		collectable[name] = true
		collection.Environments = append(collection.Environments,
			core.Abandoned{Name: name, Root: root})
	}

	for name := range unlabelled {
		collection.Unattributable.Environments = append(collection.Unattributable.Environments,
			core.Abandoned{Name: name, Reason: "no hm labels"})
	}

	if err := c.volumes(&collection, collectable, projects); err != nil {
		return collection, err
	}

	if err := c.worktrees(&collection); err != nil {
		return collection, err
	}

	collection.Hosts = c.hosts(projects)

	sortCollection(&collection)

	return collection, nil
}

// volumes are attributed through their project's containers. A volume whose project has no
// containers left could belong to anything, so it is listed and left alone.
func (c Collector) volumes(collection *core.Collection, collectable, projects map[string]bool) error {
	all, err := c.Volumes.All()
	if err != nil {
		return err
	}

	for _, volume := range all {
		labels := c.Volumes.Labels(volume)
		project := labels["com.docker.compose.project"]

		//
		// A frozen data directory belongs to no compose project, so the rule above cannot see it
		// — and they are the largest volumes this tool makes. It carries the project that made it
		// and where that project lived, which is the same question asked of everything else here.
		//
		if project == "" {
			if labels[core.TemplateLabel] == "" || labels["hm.root"] == "" {
				continue
			}

			if !c.FS.IsDir(labels["hm.root"]) {
				collection.Volumes = append(collection.Volumes, volume)
			}

			continue
		}

		switch {
		case collectable[project]:
			collection.Volumes = append(collection.Volumes, volume)
		case !projects[project]:
			collection.Unattributable.Volumes = append(collection.Unattributable.Volumes, volume)
		}
	}

	return nil
}

// worktrees whose directory is gone.
//
// Their containers and volumes are collected by the rules above already — they carry the root, and
// that directory no longer exists. What is left is the registration, which nothing else deletes:
// `worktree remove` is the tidy path and it needs the directory to still be there. Somebody who
// removes a worktree with git leaves an entry that says "missing" for ever and refuses the name if
// they ever want it back.
func (c Collector) worktrees(collection *core.Collection) error {
	parents, err := c.Registry.Parents()
	if err != nil {
		return err
	}

	for _, parent := range parents {
		registered, err := c.Registry.Worktrees(parent)
		if err != nil {
			continue
		}

		for _, worktree := range registered {
			if c.FS.IsDir(worktree.Path) {
				continue
			}

			collection.Worktrees = append(collection.Worktrees, core.AbandonedWorktree{
				Name: parent + "/" + worktree.Name, Project: worktree.Project, Path: worktree.Path,
			})
		}
	}

	return nil
}

// hosts are the entries this tool wrote whose environment is gone. A domain belongs to a live
// environment when some project on the machine is named after its first label, which is how this
// tool builds them.
func (c Collector) hosts(projects map[string]bool) []string {
	orphans := []string{}

	for _, domain := range c.Machine.MarkedHosts(c.Marker) {
		if projects[strings.SplitN(domain, ".", 2)[0]] {
			continue
		}

		orphans = append(orphans, domain)
	}

	return orphans
}

// Collect deletes what the survey found, and nothing else.
func (c Collector) Collect(collection core.Collection) error {
	containers, err := c.Engine.Containers()
	if err != nil {
		return err
	}

	for _, environment := range collection.Environments {
		if err := c.Engine.Remove(idsOf(containers, func(one core.Container) bool {
			return one.Project == environment.Name
		})); err != nil {
			return err
		}
	}

	//
	// A branch environment's own containers and volumes are deleted by name, not by asking
	// Compose: the directory that held its configuration is exactly what is missing.
	//
	for _, worktree := range collection.Worktrees {
		if err := c.Engine.Remove(idsOf(containers, func(one core.Container) bool {
			return one.ComposeProject == worktree.Project
		})); err != nil {
			return err
		}

		all, err := c.Volumes.All()
		if err != nil {
			return err
		}

		for _, volume := range all {
			if strings.HasPrefix(volume, worktree.Project+"_") {
				c.Volumes.Remove(volume) //nolint:errcheck
			}
		}

		parent, name, _ := strings.Cut(worktree.Name, "/")

		if err := c.Registry.Forget(parent, name); err != nil {
			return err
		}
	}

	for _, volume := range collection.Volumes {
		c.Volumes.Remove(volume) //nolint:errcheck
	}

	return nil
}

func idsOf(containers []core.Container, matches func(core.Container) bool) []string {
	ids := []string{}

	for _, container := range containers {
		if matches(container) {
			ids = append(ids, container.ID)
		}
	}

	return ids
}

// sortCollection makes the answer the same every time. A report whose lines moved between runs
// would be read as a report that changed.
func sortCollection(collection *core.Collection) {
	sort.Slice(collection.Environments, func(a, b int) bool {
		return collection.Environments[a].Name < collection.Environments[b].Name
	})
	sort.Slice(collection.Unattributable.Environments, func(a, b int) bool {
		return collection.Unattributable.Environments[a].Name < collection.Unattributable.Environments[b].Name
	})
	sort.Strings(collection.Volumes)
	sort.Strings(collection.Unattributable.Volumes)
	sort.Strings(collection.Hosts)
	sort.Slice(collection.Worktrees, func(a, b int) bool {
		return collection.Worktrees[a].Name < collection.Worktrees[b].Name
	})
}
