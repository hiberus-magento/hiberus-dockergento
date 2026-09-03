package dockerd

import (
	"context"
	"sort"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// Volumes is the Docker volumes on this machine.
type Volumes struct {
	Timeout time.Duration
}

// Labelled returns every volume carrying a label, with the labels it carries and its name under
// the key "name".
func (v Volumes) Labelled(label string) ([]map[string]string, error) {
	docker, err := connect()
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	listed, err := docker.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", label)),
	})
	if err != nil {
		return nil, err
	}

	found := make([]map[string]string, 0, len(listed.Volumes))

	for _, one := range listed.Volumes {
		labels := map[string]string{"name": one.Name}

		for key, value := range one.Labels {
			labels[key] = value
		}

		found = append(found, labels)
	}

	return found, nil
}

func (v Volumes) Exists(name string) bool {
	return v.Labels(name) != nil
}

// Labels of one volume, or nil when there is no such volume.
func (v Volumes) Labels(name string) map[string]string {
	docker, err := connect()
	if err != nil {
		return nil
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	found, err := docker.VolumeInspect(ctx, name)
	if err != nil {
		return nil
	}

	labels := map[string]string{}
	for key, value := range found.Labels {
		labels[key] = value
	}

	return labels
}

func (v Volumes) Create(name string, labels map[string]string) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	_, err = docker.VolumeCreate(ctx, volume.CreateOptions{Name: name, Labels: labels})

	return err
}

func (v Volumes) Remove(name string) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	return docker.VolumeRemove(ctx, name, false)
}

// All is every volume on this machine, whether this tool made it or not.
func (v Volumes) All() ([]string, error) {
	docker, err := connect()
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	listed, err := docker.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(listed.Volumes))

	for _, one := range listed.Volumes {
		names = append(names, one.Name)
	}

	sort.Strings(names)

	return names, nil
}

// Users are the containers holding a volume, of any state.
func (v Volumes) Users(name string) ([]string, error) {
	docker, err := connect()
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	ctx, cancel := v.deadline()
	defer cancel()

	listed, err := docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("volume", name)),
	})
	if err != nil {
		return nil, err
	}

	users := make([]string, 0, len(listed))

	for _, one := range listed {
		if len(one.Names) > 0 {
			users = append(users, trimSlash(one.Names[0]))
		}
	}

	return users, nil
}

func trimSlash(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}

	return name
}

func (v Volumes) deadline() (context.Context, context.CancelFunc) {
	timeout := v.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return context.WithTimeout(context.Background(), timeout)
}
