package dockerd

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Address is where a container can be reached: its address on every network it is attached to,
// one after the other in the order of the networks' names, which is what
// `docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'` prints — the
// template ranges over the map in key order and puts nothing between the addresses.
func (e Engine) Address(id string) (string, error) {
	docker, err := connect()
	if err != nil {
		return "", err
	}
	defer docker.Close()

	timeout := e.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	inspected, err := docker.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}

	if inspected.NetworkSettings == nil {
		return "", nil
	}

	names := make([]string, 0, len(inspected.NetworkSettings.Networks))
	for name := range inspected.NetworkSettings.Networks {
		names = append(names, name)
	}

	sort.Strings(names)

	var address strings.Builder

	for _, name := range names {
		if endpoint := inspected.NetworkSettings.Networks[name]; endpoint != nil {
			address.WriteString(endpoint.IPAddress)
		}
	}

	return address.String(), nil
}
