package app

import (
	"sort"
	"sync"

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

		// A directory that is gone is what tells an abandoned environment from a stopped one
		if environment.Root != "" && !i.FS.IsDir(environment.Root) {
			environment.Orphan = true
		}

		environments = append(environments, environment)
	}

	i.resolveBranches(environments)

	sort.Slice(environments, func(a, b int) bool {
		return environments[a].Name < environments[b].Name
	})

	return environments, nil
}

// resolveBranches asks git for the branch of every environment at once.
//
// Asking is a subprocess each, and on a machine with a dozen environments that was most of the
// time the command took — in the shell implementation it was unavoidable, one after another. Here
// they are independent questions with no shared state, which is the plainest case there is for
// doing them at the same time.
//
// Bounded, because a machine with sixty environments spawning sixty gits at once helps nobody,
// and the order is settled afterwards so the answer does not depend on which one finished first.
func (i Inventory) resolveBranches(environments []core.Environment) {
	const atOnce = 8

	permits := make(chan struct{}, atOnce)
	var waiting sync.WaitGroup

	for index := range environments {
		// A directory that is not there has no branch, and asking would be a subprocess for
		// nothing
		if environments[index].Root == "" || environments[index].Orphan {
			continue
		}

		waiting.Add(1)
		permits <- struct{}{}

		go func(at int) {
			defer waiting.Done()
			defer func() { <-permits }()

			environments[at].Branch = i.Branches.Branch(environments[at].Root)
		}(index)
	}

	waiting.Wait()
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
