package dockerd

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// Runner runs a command inside a container and gives back what it said.
//
// Through the API rather than by shelling out to `docker exec`, and capturing rather than
// attaching: the answer to a query is something a caller reads, not something a terminal shows.
type Runner struct {
	Timeout time.Duration
}

// Run executes the command and returns its own exit code.
func (r Runner) Run(id string, command []string, environment []string, out io.Writer) (int, error) {
	docker, err := connect()
	if err != nil {
		return 0, err
	}
	defer docker.Close()

	timeout := r.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	created, err := docker.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          command,
		Env:          environment,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return 0, err
	}

	attached, err := docker.ContainerExecAttach(ctx, created.ID, container.ExecStartOptions{})
	if err != nil {
		return 0, err
	}
	defer attached.Close()

	// Without a terminal the daemon multiplexes stdout and stderr into one stream with a header
	// per chunk, so it has to be taken apart rather than copied
	if _, err := stdcopy.StdCopy(out, out, attached.Reader); err != nil {
		return 0, err
	}

	inspected, err := docker.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return 0, err
	}

	return inspected.ExitCode, nil
}
