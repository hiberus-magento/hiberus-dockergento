package app

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// A service of the project reachable from this machine, for as long as somebody holds it open.
//
// Nothing in the environment publishes a port, so that several can be up at once; a database
// client or a debugger that needs one gets it here, on a free port, through a relay on the
// service's network that is published on 127.0.0.1 and nowhere else.
//

const (
	tunnelImage = "alpine/socat"
	tunnelHost  = "127.0.0.1"

	// tunnelTries is how many times the forwarded port is tried before giving up, tunnelEvery apart
	tunnelTries = 20
	tunnelEvery = 250 * time.Millisecond
)

// Tunneler forwards a service of a project to a port of this machine.
type Tunneler struct {
	Engine   ports.ContainerEngine
	Relays   ports.Relays
	Services ports.ServiceLister
	Exposure ports.Exposure

	// Binary is the name the tool was invoked as, for the hints.
	Binary string

	// FreePort finds a port of this machine nobody listens on, and Reachable says whether
	// something answers on one now.
	FreePort  func() (int, error)
	Reachable func(port int) bool

	// Sleep is the wait between two tries of the forwarded port. Nil is time.Sleep.
	Sleep func(time.Duration)
}

// tunnelRelay is the name of the container forwarding a project's service; without a service it
// is the prefix every one of the project's carries.
func tunnelRelay(project core.Project, service string) string {
	name := "hm-tunnel-" + project.Name
	if service != "" {
		name += "-" + service
	}

	return name
}

// Open forwards service to a free port of this machine. port is the service's port to forward;
// zero means the only one it exposes.
//
// What it opens stays open when it returns: the caller holds it with Wait and ends it with Close.
func (t Tunneler) Open(project core.Project, files core.ComposeFiles, service string, port int) (core.Tunnel, error) {
	if service == "" {
		return core.Tunnel{}, core.Refusal{
			Kind: "missing_service", Code: 2,
			Message: "Which service should be reachable?",
			Hint:    t.Binary + " tunnel db",
		}
	}

	available, err := t.Services.Services(project, files)
	if err != nil {
		return core.Tunnel{}, err
	}

	sort.Strings(available)

	if !slices.Contains(available, service) {
		return core.Tunnel{}, core.Refusal{
			Kind: "unknown_service", Code: 5,
			Message: fmt.Sprintf("This project has no service called '%s'", service),
			Hint:    t.Binary + " tunnel " + strings.Join(available, " "),
		}
	}

	target, err := runningContainer(t.Engine, project, service, t.Binary)
	if err != nil {
		return core.Tunnel{}, err
	}

	if port == 0 {
		exposed, err := t.Exposure.Exposed(target.ID)
		if err != nil {
			return core.Tunnel{}, err
		}

		sort.Ints(exposed)

		if len(exposed) != 1 {
			listed := make([]string, len(exposed))
			for index, one := range exposed {
				listed[index] = strconv.Itoa(one)
			}

			return core.Tunnel{}, core.Refusal{
				Kind: "ambiguous_port", Code: 2,
				Message: fmt.Sprintf("'%s' exposes %d ports, so the one to forward has to be said",
					service, len(exposed)),
				Hint: fmt.Sprintf("%s tunnel %s %s", t.Binary, service, strings.Join(listed, " ")),
			}
		}

		port = exposed[0]
	}

	local, err := t.FreePort()
	if err != nil {
		return core.Tunnel{}, core.Refusal{
			Kind: "no_free_port", Code: 1,
			Message: "No free local port could be found",
			Hint:    fmt.Sprintf("%s tunnel %s %d", t.Binary, service, port),
		}
	}

	network := ""
	if len(target.Networks) > 0 {
		network = target.Networks[0]
	}

	relay := tunnelRelay(project, service)

	// Whatever a closed window left behind for this service
	t.Relays.Remove(relay, true) //nolint:errcheck

	if err := t.Relays.Start(core.Relay{
		Name:    relay,
		Image:   tunnelImage,
		Network: network,
		Command: []string{
			fmt.Sprintf("tcp-listen:%d,fork,reuseaddr", port),
			fmt.Sprintf("tcp-connect:%s:%d", service, port),
		},
		HostIP:        tunnelHost,
		HostPort:      local,
		ContainerPort: port,
		AutoRemove:    true,
	}); err != nil {
		return core.Tunnel{}, core.Refusal{
			Kind: "tunnel_failed", Code: 3,
			Message: "The tunnel could not be opened",
			Hint:    "docker pull " + tunnelImage,
		}
	}

	if !t.answers(local) {
		t.Relays.Remove(relay, false) //nolint:errcheck

		return core.Tunnel{}, core.Refusal{
			Kind: "tunnel_not_ready", Code: 1,
			Message: fmt.Sprintf("The tunnel was opened but nothing is answering on %s:%d", tunnelHost, local),
			Hint:    "docker logs " + relay,
		}
	}

	return core.Tunnel{Service: service, Host: tunnelHost, Port: local, ContainerPort: port, Relay: relay}, nil
}

// Wait returns when the tunnel stops, which is when somebody closed it.
func (t Tunneler) Wait(relay string) error {
	return t.Relays.Wait(relay)
}

// Close takes away the project's tunnels to service, or every one of them without a service, and
// reports whether there was any.
func (t Tunneler) Close(project core.Project, service string) (bool, error) {
	return t.Relays.Remove(tunnelRelay(project, service), true)
}

func (t Tunneler) answers(port int) bool {
	for try := 0; try < tunnelTries; try++ {
		if t.Reachable(port) {
			return true
		}

		if t.Sleep != nil {
			t.Sleep(tunnelEvery)
		} else {
			time.Sleep(tunnelEvery)
		}
	}

	return false
}
