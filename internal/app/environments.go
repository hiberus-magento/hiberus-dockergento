package app

import (
	"sort"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core/ports"
)

// Inventory answers what environments exist on this machine.
type Inventory struct {
	Engine   ports.ContainerEngine
	FS       ports.FS
	Branches ports.Branches
}

// Environments groups every container into the environment it belongs to.
//
// What counts as one of ours is deliberately generous: a container carrying our labels, or a
// `phpfpm` service of a Compose project. The second is what keeps environments created before the
// labels existed from disappearing from the inventory — and disappearing from an inventory is how
// something never gets cleaned up.
func (i Inventory) Environments() ([]core.Environment, error) {
	containers, err := i.Engine.Containers()
	if err != nil {
		return nil, err
	}

	type accumulator struct {
		environment core.Environment
		ours        bool
		hasPHP      bool
	}

	grouped := map[string]*accumulator{}

	for _, container := range containers {
		key := container.Key()
		if key == "" {
			continue
		}

		entry, seen := grouped[key]
		if !seen {
			entry = &accumulator{}
			entry.environment.Name = key
			grouped[key] = entry
		}

		if container.Project != "" {
			entry.ours = true
			entry.environment.HasMetadata = true
		}

		if container.ComposeService == "phpfpm" {
			entry.hasPHP = true
		}

		entry.environment.Containers.Total++
		if container.Running {
			entry.environment.Containers.Running++
		}

		// The first non-empty wins: containers of one environment carry the same values, and a
		// later one with the label missing must not erase what an earlier one knew
		if entry.environment.Root == "" {
			if container.Root != "" {
				entry.environment.Root = container.Root
			} else {
				entry.environment.Root = container.WorkingDir
			}
		}

		if entry.environment.Worktree == "" {
			entry.environment.Worktree = container.Worktree
		}

		if entry.environment.Magento == "" {
			entry.environment.Magento = container.Magento
		}
	}

	environments := make([]core.Environment, 0, len(grouped))

	for _, entry := range grouped {
		if !entry.ours && !entry.hasPHP {
			continue
		}

		environment := entry.environment
		environment.Status = statusOf(environment.Containers.Running, environment.Containers.Total)

		// A directory that is gone is what tells an abandoned environment from a stopped one.
		// Asking git for the branch of a directory that is not there would be a subprocess per
		// environment for nothing.
		if environment.Root != "" {
			if i.FS.IsDir(environment.Root) {
				environment.Branch = i.Branches.Branch(environment.Root)
			} else {
				environment.Orphan = true
			}
		}

		environments = append(environments, environment)
	}

	sort.Slice(environments, func(a, b int) bool {
		return environments[a].Name < environments[b].Name
	})

	return environments, nil
}

func statusOf(running, total int) string {
	switch {
	case running == 0:
		return "stopped"
	case running == total:
		return "running"
	default:
		return "partial"
	}
}
