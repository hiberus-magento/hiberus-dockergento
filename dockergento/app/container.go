package app

import (
	"fmt"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// resolveContainer is the container holding this project's given service, and refusing when it is
// not running is the whole point: every other answer would be a connection error three layers
// down. Shared by every caller that needs to turn a service name into a container id, so that a
// second hardcoded copy of this resolution never has the chance to drift from the first.
func resolveContainer(engine ports.ContainerEngine, project core.Project, service, binary string) (string, error) {
	found, err := runningContainer(engine, project, service, binary)

	return found.ID, err
}

// runningContainer is resolveContainer for a caller that needs more of the container than its id:
// the networks it is attached to, among others.
func runningContainer(engine ports.ContainerEngine, project core.Project, service, binary string) (core.Container, error) {
	running, _, err := serviceContainers(engine, project, service)
	if err != nil {
		return core.Container{}, err
	}

	if running == nil {
		return core.Container{}, core.Refusal{
			Kind:    "service_not_running",
			Code:    5,
			Message: fmt.Sprintf("Service '%s' is not running", service),
			Hint:    fmt.Sprintf("%s start %s", binary, service),
		}
	}

	return *running, nil
}

// anyContainer is runningContainer for a caller that is served by a stopped container as well:
// copying out of one is a read of its filesystem, which the daemon serves either way. A running
// one still wins, so an orphaned stopped container is never read instead of the live one; a
// stopped one is taken only when it is all there is, and having none is refused as not there
// rather than as stopped.
func anyContainer(engine ports.ContainerEngine, project core.Project, service, binary string) (core.Container, error) {
	running, stopped, err := serviceContainers(engine, project, service)
	if err != nil {
		return core.Container{}, err
	}

	switch {
	case running != nil:
		return *running, nil
	case stopped != nil:
		return *stopped, nil
	}

	return core.Container{}, core.Refusal{
		Kind:    "service_not_running",
		Code:    5,
		Message: fmt.Sprintf("This project has no %s container", service),
		Hint:    fmt.Sprintf("%s start %s", binary, service),
	}
}

// serviceContainers is the first running and the first stopped container of the service. Every
// match is looked at: an orphaned stopped container listed before the live one is not a reason to
// miss the live one.
func serviceContainers(engine ports.ContainerEngine, project core.Project, service string) (running, stopped *core.Container, err error) {
	containers, err := engine.Containers()
	if err != nil {
		return nil, nil, err
	}

	for at, candidate := range containers {
		if candidate.Key() != project.Name || candidate.ComposeService != service {
			continue
		}

		if candidate.Running {
			return &containers[at], stopped, nil
		}

		if stopped == nil {
			stopped = &containers[at]
		}
	}

	return nil, stopped, nil
}
