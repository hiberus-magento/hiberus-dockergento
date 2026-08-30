package dockerd

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// Daemon answers what the diagnosis needs to know about Docker itself.
type Daemon struct {
	// Timeout bounds every call, for the same reason the engine's does: a daemon that is starting
	// answers eventually or never, and a diagnosis that hangs is worse than one that fails.
	Timeout time.Duration
}

// Reachable is the first question the diagnosis asks.
func (d Daemon) Reachable() bool {
	docker, err := connect()
	if err != nil {
		return false
	}
	defer docker.Close()

	ctx, cancel := d.deadline()
	defer cancel()

	_, err = docker.Ping(ctx)

	return err == nil
}

// Info is what the daemon says about itself.
func (d Daemon) Info() (core.DaemonInfo, error) {
	docker, err := connect()
	if err != nil {
		return core.DaemonInfo{}, err
	}
	defer docker.Close()

	ctx, cancel := d.deadline()
	defer cancel()

	info, err := docker.Info(ctx)
	if err != nil {
		return core.DaemonInfo{}, err
	}

	return core.DaemonInfo{
		MemoryBytes: info.MemTotal,
		CPUs:        info.NCPU,
		Runtime:     info.Name,
	}, nil
}

// Leftovers counts volumes and dangling images.
func (d Daemon) Leftovers() (int, int, error) {
	docker, err := connect()
	if err != nil {
		return 0, 0, err
	}
	defer docker.Close()

	ctx, cancel := d.deadline()
	defer cancel()

	volumes, err := docker.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return 0, 0, err
	}

	dangling, err := docker.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("dangling", "true")),
	})
	if err != nil {
		return len(volumes.Volumes), 0, nil
	}

	return len(volumes.Volumes), len(dangling), nil
}

// ImageAvailability reports whether an image is here and whether it could be pulled.
//
// The second question is asked of the registry, which is the point: an image that was published
// by hand and never pushed is one Docker cannot pull, and the natural failure for that is a
// half-created environment at `up` time, in Docker's own wording.
func (d Daemon) ImageAvailability(reference string) (bool, bool) {
	docker, err := connect()
	if err != nil {
		return false, false
	}
	defer docker.Close()

	ctx, cancel := d.deadline()
	defer cancel()

	if _, _, err := docker.ImageInspectWithRaw(ctx, reference); err == nil {
		return true, true
	}

	if _, err := docker.DistributionInspect(ctx, reference, ""); err == nil {
		return false, true
	}

	return false, false
}

func (d Daemon) deadline() (context.Context, context.CancelFunc) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return context.WithTimeout(context.Background(), timeout)
}

// connect builds a client pointed at wherever the daemon actually is.
//
// Resolved from the docker context when the environment does not say. Without it the binary
// reports "Docker is not running" on every machine using Colima or Docker Desktop, which is all
// of them.
func connect() (*client.Client, error) {
	options := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}

	if host := Endpoint(); host != "" {
		options = append(options, client.WithHost(host))
	}

	return client.NewClientWithOpts(options...)
}
