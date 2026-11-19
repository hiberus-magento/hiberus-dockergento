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
	containers, err := engine.Containers()
	if err != nil {
		return core.Container{}, err
	}

	for _, candidate := range containers {
		if candidate.Key() != project.Name || candidate.ComposeService != service {
			continue
		}

		if !candidate.Running {
			break
		}

		return candidate, nil
	}

	return core.Container{}, core.Refusal{
		Kind:    "service_not_running",
		Code:    5,
		Message: fmt.Sprintf("Service '%s' is not running", service),
		Hint:    fmt.Sprintf("%s start %s", binary, service),
	}
}
