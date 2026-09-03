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

// Capture runs a command whose output is being kept, and returns its exit code.
//
// Two things separate it from Run, and they are the same case: what the command writes to its
// error stream must not land in the output, because the output is a file being written; and there
// is no deadline, because a dump of a real database takes minutes and a cut-off one looks
// finished.
func (r Runner) Capture(id string, command []string, out, errors io.Writer) (int, error) {
	docker, err := connect()
	if err != nil {
		return 0, err
	}
	defer docker.Close()

	ctx := context.Background()

	created, err := docker.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          command,
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

	if _, err := stdcopy.StdCopy(out, errors, attached.Reader); err != nil {
		return 0, err
	}

	inspected, err := docker.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return 0, err
	}

	return inspected.ExitCode, nil
}

// Feed sends a stream into the command's input and returns its exit code.
//
// The dump of a Magento database is measured in gigabytes, so it is written into the connection as
// it is read rather than assembled anywhere. The write side has to be closed when the stream ends:
// the client waits for end-of-input and would otherwise sit there for ever.
func (r Runner) Feed(id string, command []string, in io.Reader, out io.Writer) (int, error) {
	docker, err := connect()
	if err != nil {
		return 0, err
	}
	defer docker.Close()

	// No deadline here, unlike a query: importing a real dump takes as long as it takes, and a
	// timeout would leave a database half replaced
	ctx := context.Background()

	created, err := docker.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          command,
		AttachStdin:  true,
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

	sending := make(chan error, 1)

	go func() {
		_, err := io.Copy(attached.Conn, in)
		attached.CloseWrite() //nolint:errcheck
		sending <- err
	}()

	if _, err := stdcopy.StdCopy(out, out, attached.Reader); err != nil {
		return 0, err
	}

	if err := <-sending; err != nil {
		return 0, err
	}

	inspected, err := docker.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return 0, err
	}

	return inspected.ExitCode, nil
}
