package dockerd

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// OneOff runs a container that belongs to no environment and removes it afterwards.
type OneOff struct {
	// Out is where what it says goes. Empty means this process's output.
	Out io.Writer
}

// Run pulls the image if it is not here, runs it attached to the given network, and removes it.
func (o OneOff) Run(reference string, command []string, attached string, binds []core.Bind, tty bool) (int, error) {
	docker, err := connect()
	if err != nil {
		return 0, err
	}
	defer docker.Close()

	ctx := context.Background()

	// An image that is not here is not an error to report to somebody: it is a pull
	if _, _, err := docker.ImageInspectWithRaw(ctx, reference); err != nil {
		pulled, err := docker.ImagePull(ctx, reference, image.PullOptions{})
		if err != nil {
			return 0, err
		}

		io.Copy(io.Discard, pulled) //nolint:errcheck
		pulled.Close()              //nolint:errcheck
	}

	mounts := make([]mount.Mount, 0, len(binds))

	for _, bind := range binds {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeBind, Source: bind.Source, Target: bind.Target,
		})
	}

	networks := map[string]*network.EndpointSettings{}
	if attached != "" {
		networks[attached] = &network.EndpointSettings{}
	}

	created, err := docker.ContainerCreate(ctx,
		&container.Config{Image: reference, Cmd: command, Tty: tty,
			AttachStdout: true, AttachStderr: true},
		&container.HostConfig{Mounts: mounts, AutoRemove: false},
		&network.NetworkingConfig{EndpointsConfig: networks},
		nil, "")
	if err != nil {
		return 0, err
	}

	// Removed whatever happens, including when this returns early: a tool that runs for a minute
	// and leaves a container behind is a tool that fills a machine
	defer docker.ContainerRemove(ctx, created.ID, //nolint:errcheck
		container.RemoveOptions{Force: true})

	logs, err := docker.ContainerAttach(ctx, created.ID, container.AttachOptions{
		Stream: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return 0, err
	}
	defer logs.Close()

	if err := docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return 0, err
	}

	out := o.Out
	if out == nil {
		out = os.Stdout
	}

	if tty {
		io.Copy(out, logs.Reader) //nolint:errcheck
	} else {
		stdcopy.StdCopy(out, out, logs.Reader) //nolint:errcheck
	}

	waiting, failed := docker.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)

	select {
	case err := <-failed:
		return 0, err
	case status := <-waiting:
		return int(status.StatusCode), nil
	}
}

// Volumes runs a container with volumes attached and gives back what it said.
//
// Volumes and not directories of this machine, because on macOS a volume lives inside a virtual
// machine and its mountpoint does not exist out here: measuring or copying a data directory has
// to happen from inside something that can see it.
func (o OneOff) Volumes(reference string, command []string, mounts map[string]string, out io.Writer) (int, error) {
	docker, err := connect()
	if err != nil {
		return 0, err
	}
	defer docker.Close()

	ctx := context.Background()

	attached := make([]mount.Mount, 0, len(mounts))

	for name, target := range mounts {
		readOnly := strings.HasSuffix(target, ":ro")

		attached = append(attached, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   name,
			Target:   strings.TrimSuffix(target, ":ro"),
			ReadOnly: readOnly,
		})
	}

	created, err := docker.ContainerCreate(ctx,
		&container.Config{Image: reference, Cmd: command, Entrypoint: []string{"/bin/bash"},
			AttachStdout: true, AttachStderr: true},
		&container.HostConfig{Mounts: attached},
		nil, nil, "")
	if err != nil {
		return 0, err
	}

	defer docker.ContainerRemove(ctx, created.ID, //nolint:errcheck
		container.RemoveOptions{Force: true})

	logs, err := docker.ContainerAttach(ctx, created.ID, container.AttachOptions{
		Stream: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return 0, err
	}
	defer logs.Close()

	if err := docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return 0, err
	}

	if out == nil {
		out = io.Discard
	}

	stdcopy.StdCopy(out, io.Discard, logs.Reader) //nolint:errcheck

	waiting, failed := docker.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)

	select {
	case err := <-failed:
		return 0, err
	case status := <-waiting:
		return int(status.StatusCode), nil
	}
}
