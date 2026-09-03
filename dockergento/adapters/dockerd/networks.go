package dockerd

import (
	"context"

	"github.com/docker/docker/api/types/network"
)

// Networks are the Docker networks on this machine.
//
// Only the shared one the proxy routes through is managed here: a project's own networks are
// Compose's business and creating them behind its back is how a network ends up owned by nobody.
type Networks struct{}

// Exists reports whether a network is there.
func (Networks) Exists(name string) bool {
	docker, err := connect()
	if err != nil {
		return false
	}
	defer docker.Close()

	_, err = docker.NetworkInspect(context.Background(), name, network.InspectOptions{})

	return err == nil
}

// Ensure creates the network if it is not there, and says nothing when it already is.
//
// Created before anything tries to join it, which is the whole reason this is not left to Compose:
// the projects declare it as external, so the first one to start would fail on a machine where the
// proxy had never run.
func (n Networks) Ensure(name string) error {
	if n.Exists(name) {
		return nil
	}

	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	_, err = docker.NetworkCreate(context.Background(), name, network.CreateOptions{})

	// Two commands creating it at the same moment is not a failure: what was asked for is that it
	// exists, and it does
	if err != nil && n.Exists(name) {
		return nil
	}

	return err
}
