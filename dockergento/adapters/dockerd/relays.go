package dockerd

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Relays are the containers that hold a tunnel or a forwarded port open: no environment's, found
// by name.
type Relays struct{}

// Remove takes away, forcing them, the containers named name, or whose name starts with it.
func (Relays) Remove(name string, prefix bool) (bool, error) {
	docker, err := connect()
	if err != nil {
		return false, err
	}
	defer docker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The name filter is a regular expression over the name, as `docker ps --filter name=` takes it
	pattern := "^/?" + name
	if !prefix {
		pattern += "$"
	}

	listed, err := docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", pattern)),
	})
	if err != nil {
		return false, err
	}

	for _, one := range listed {
		if err := docker.ContainerRemove(ctx, one.ID, container.RemoveOptions{Force: true}); err != nil {
			return true, err
		}
	}

	return len(listed) > 0, nil
}

// Start creates and starts a relay, detached, pulling its image first when it is not here — which
// `docker run` does by itself and the API does not.
func (Relays) Start(relay core.Relay) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	ctx := context.Background()

	if _, _, err := docker.ImageInspectWithRaw(ctx, relay.Image); err != nil {
		pulled, err := docker.ImagePull(ctx, relay.Image, image.PullOptions{})
		if err != nil {
			return err
		}

		io.Copy(io.Discard, pulled) //nolint:errcheck
		pulled.Close()              //nolint:errcheck
	}

	config := &container.Config{Image: relay.Image, Cmd: relay.Command}
	host := &container.HostConfig{AutoRemove: relay.AutoRemove}

	if relay.HostPort != 0 {
		port := nat.Port(strconv.Itoa(relay.ContainerPort) + "/tcp")
		config.ExposedPorts = nat.PortSet{port: struct{}{}}
		host.PortBindings = nat.PortMap{port: []nat.PortBinding{
			{HostIP: relay.HostIP, HostPort: strconv.Itoa(relay.HostPort)},
		}}
	}

	networks := map[string]*network.EndpointSettings{}
	if relay.Network != "" {
		networks[relay.Network] = &network.EndpointSettings{}
	}

	created, err := docker.ContainerCreate(ctx, config, host,
		&network.NetworkingConfig{EndpointsConfig: networks}, nil, relay.Name)
	if err != nil {
		return err
	}

	if err := docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		docker.ContainerRemove(ctx, created.ID, container.RemoveOptions{Force: true}) //nolint:errcheck

		return err
	}

	return nil
}

// Logs is what a relay has written so far, both streams together, as `docker logs name 2>&1`.
func (Relays) Logs(name string) (string, error) {
	docker, err := connect()
	if err != nil {
		return "", err
	}
	defer docker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logs, err := docker.ContainerLogs(ctx, name, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}
	defer logs.Close()

	var both bytes.Buffer

	stdcopy.StdCopy(&both, &both, logs) //nolint:errcheck

	return both.String(), nil
}

// Wait returns when a relay stops, or is removed. A relay that is already gone is not an error:
// that is what closing it from somewhere else looks like.
func (Relays) Wait(name string) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	waiting, failed := docker.ContainerWait(context.Background(), name, container.WaitConditionNotRunning)

	select {
	case <-failed:
		return nil
	case <-waiting:
		return nil
	}
}
