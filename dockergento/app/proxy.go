package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Proxy is the one router on this machine: it holds 80 and 443 and sends each request to the
// project whose domain it carries.
//
// Everything here is about the proxy itself. What a *project* has to look like to be routed is the
// overlay, and that is written where the project is set up.
type Proxy struct {
	Engine       ports.ContainerEngine
	Networks     ports.NetworkStore
	Orchestrator ports.Orchestrator
	Tooling      ports.Tooling

	// Routes asks the proxy what it is routing, which is a question only the proxy can answer.
	Routes func() ([]core.Route, error)

	// Dir is where its compose file and its certificates live.
	Dir string

	Announce func(string)
	Binary   string
}

// Up starts it, and says whether it had to.
func (p Proxy) Up() (bool, error) {
	if !core.ComposeAtLeast(p.Tooling.ComposeVersion(), core.ProxyMinCompose) {
		return false, core.Refusal{
			Kind: "compose_too_old",
			Code: 1,
			Message: fmt.Sprintf("The proxy needs Docker Compose %s or newer, and this is %s",
				core.ProxyMinCompose, p.Tooling.ComposeVersion()),
			Hint: "Update Docker Desktop, or leave the proxy off in your projects",
		}
	}

	running, holder, err := p.state()
	if err != nil {
		return false, err
	}

	if running {
		return false, nil
	}

	//
	// The proxy listens on 80 and 443; a project that does not use it publishes those itself. So
	// the two cannot be up at the same time, and saying which project is in the way is far more
	// useful than Docker's "port is already allocated".
	//
	if holder != "" {
		return false, core.Refusal{
			Kind:    "ports_taken",
			Code:    6,
			Message: fmt.Sprintf("'%s' is already using port 80 or 443, which the proxy needs", holder),
			Hint:    "Stop that environment first: it does not go through the proxy",
		}
	}

	if err := p.Networks.Ensure(core.ProxyNetwork); err != nil {
		return false, err
	}

	if err := p.write(); err != nil {
		return false, err
	}

	p.say("Starting the proxy...\n")

	if err := p.Orchestrator.Up(p.project(), p.files(), nil); err != nil {
		return false, err
	}

	running, _, err = p.state()
	if err != nil {
		return false, err
	}

	if !running {
		return false, core.Refusal{
			Kind:    "proxy_failed",
			Code:    3,
			Message: "The proxy did not start",
			Hint: fmt.Sprintf("docker compose -p %s -f %s logs",
				core.ProxyProject, filepath.Join(p.Dir, "docker-compose.yml")),
		}
	}

	return true, nil
}

// Down stops it, and says whether it had to.
//
// Stopping it takes every routed site down with it, which is worth saying out loud rather than
// leaving somebody to discover.
func (p Proxy) Down() (bool, error) {
	running, _, err := p.state()
	if err != nil {
		return false, err
	}

	if !running {
		return false, nil
	}

	if _, err := os.Stat(filepath.Join(p.Dir, "docker-compose.yml")); err != nil {
		return false, nil //nolint:nilerr
	}

	p.say("Stopping the proxy. Any project routed through it becomes unreachable.\n")

	return true, p.Orchestrator.Down(p.project(), p.files(), core.DownOptions{})
}

// Status is whether it is running and what it is routing.
//
// The routes come from the proxy's own API rather than from our idea of what should be routed: a
// container with the labels and a router that never came up look identical from outside.
func (p Proxy) Status() (core.ProxyState, error) {
	running, _, err := p.state()
	if err != nil {
		return core.ProxyState{}, err
	}

	state := core.ProxyState{Running: running, Network: core.ProxyNetwork, Routes: []core.Route{}}

	if !running || p.Routes == nil {
		return state, nil
	}

	routes, err := p.Routes()
	if err != nil {
		return state, nil //nolint:nilerr
	}

	state.Routes = routes

	return state, nil
}

// state is whether the proxy is running and, when it is not, who is holding the ports it needs.
func (p Proxy) state() (bool, string, error) {
	containers, err := p.Engine.Containers()
	if err != nil {
		return false, "", err
	}

	for _, container := range containers {
		if container.Running && container.Name == core.ProxyContainer {
			return true, "", nil
		}
	}

	return false, holderOf(containers, core.ProxyPorts), nil
}

// write puts the proxy's compose file where Compose will read it, and only when it changed.
//
// Through a temporary and a rename: this file is shared by every project on the machine, so
// rewriting it in place on every start is a global file being mutated while other projects may be
// reading it, for no gain.
func (p Proxy) write() error {
	for _, directory := range []string{"dynamic", "certs"} {
		if err := os.MkdirAll(filepath.Join(p.Dir, directory), 0o755); err != nil {
			return err
		}
	}

	target := filepath.Join(p.Dir, "docker-compose.yml")
	wanted := core.ProxyCompose(p.Binary)

	if current, err := os.ReadFile(target); err == nil && string(current) == wanted {
		return nil
	}

	temporary, err := os.CreateTemp(p.Dir, "docker-compose.yml.*")
	if err != nil {
		return err
	}

	if _, err := temporary.WriteString(wanted); err != nil {
		temporary.Close()           //nolint:errcheck
		os.Remove(temporary.Name()) //nolint:errcheck

		return err
	}

	if err := temporary.Close(); err != nil {
		os.Remove(temporary.Name()) //nolint:errcheck

		return err
	}

	return os.Rename(temporary.Name(), target)
}

// Certify writes the dynamic configuration that serves a domain's certificate.
//
// The certificate itself is somebody else's job — it needs a tool that can sign one — and this is
// the half that tells the proxy where to find it.
func (p Proxy) Certify(domain string) error {
	if err := os.MkdirAll(filepath.Join(p.Dir, "dynamic"), 0o755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(p.Dir, "dynamic", domain+".yml"),
		[]byte(core.ProxyCertificate(p.Binary, domain)), 0o644) //nolint:gosec
}

// project and files are the proxy seen as what it is: an ordinary compose project of its own.
func (p Proxy) project() core.Project {
	return core.Project{Name: core.ProxyProject, Root: p.Dir}
}

func (p Proxy) files() core.ComposeFiles {
	return core.ComposeFiles{Load: []string{filepath.Join(p.Dir, "docker-compose.yml")}}
}

func (p Proxy) say(message string) {
	if p.Announce != nil {
		p.Announce(message)
	}
}
