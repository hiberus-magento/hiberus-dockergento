// Package dockerd talks to the Docker daemon through its API.
//
// Through the API and not by running `docker` and reading its output: parsing the formatted
// output of a CLI is a contract nobody promised, and a single query here replaces the twelve
// column format string the shell implementation had to keep in step with three readers.
package dockerd

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// Engine is the daemon.
type Engine struct {
	// Timeout bounds every call. A daemon that is starting, or a VM that is paused, answers
	// eventually or never — and a CLI that hangs is worse than one that fails.
	Timeout time.Duration
}

// Containers returns every container on the machine, running or not.
func (e Engine) Containers() ([]core.Container, error) {
	docker, err := connect()
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	timeout := e.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	listed, err := docker.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	containers := make([]core.Container, 0, len(listed))

	for _, item := range listed {
		published := make([]string, 0, len(item.Ports))

		for _, port := range item.Ports {
			if port.PublicPort != 0 {
				published = append(published, strconv.Itoa(int(port.PublicPort)))
			}
		}

		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}

		mounts := make([]core.Mount, 0, len(item.Mounts))

		for _, mount := range item.Mounts {
			mounts = append(mounts, core.Mount{
				Type:        string(mount.Type),
				Destination: mount.Destination,
			})
		}

		containers = append(containers, core.Container{
			Mounts:         mounts,
			Name:           name,
			Published:      published,
			ID:             item.ID,
			Running:        item.State == "running",
			StateName:      item.State,
			ComposeProject: item.Labels["com.docker.compose.project"],
			ComposeService: item.Labels["com.docker.compose.service"],
			Project:        item.Labels["hm.project"],
			Root:           item.Labels["hm.root"],
			Worktree:       item.Labels["hm.worktree"],
			Magento:        item.Labels["hm.magento"],
			WorkingDir:     item.Labels["com.docker.compose.project.working_dir"],
		})
	}

	return containers, nil
}
