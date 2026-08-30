package app

import (
	"fmt"
	"path"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core/ports"
)

// Operator runs the everyday operations on an environment: bringing it up, stopping it, looking
// at what it is saying and running something inside it.
type Operator struct {
	Orchestrator ports.Orchestrator
	Engine       ports.ContainerEngine
	Legacy       ports.Legacy

	// Announce is how the steps that take a while say what they are doing before they do it.
	Announce func(string)

	// Platform is "mac" or "linux", and Binary is the name the tool was invoked as.
	Platform string
	Binary   string

	// Workdir is where the code lives inside the container, which is what the dependencies are
	// looked for under.
	Workdir string
}

// The name of the one proxy on the machine, and the ports it listens on.
const proxyContainer = "hm-proxy"

var proxyPorts = []string{"80", "443"}

// Start brings the environment up.
//
// Three things happen around the `up` itself, and each of them exists because somebody was bitten
// by its absence: the other environments can be stopped first, the proxy a project needs is
// started for it, and the dependencies are checked for being a bind mount — which on macOS is
// what makes Magento's modules invisible to itself.
func (o Operator) Start(project core.Project, files core.ComposeFiles, services []string, stopOthers, usesProxy bool) error {
	if stopOthers {
		if _, err := o.Legacy.Run([]string{"docker-stop-all"}); err != nil {
			return err
		}
	}

	if usesProxy {
		if err := o.ensureProxy(); err != nil {
			return err
		}
	}

	o.announce("Starting containers in detached mode\n\n")

	if err := o.Orchestrator.Up(project, files, services); err != nil {
		return err
	}

	// Only when the whole environment was started: naming a service is asking for that service,
	// and answering with a complaint about a different one is a command arguing with its caller
	if len(services) == 0 {
		return o.checkDependenciesAreNotBound(project)
	}

	return nil
}

// ensureProxy starts the one proxy on the machine when this project needs it and it is not up.
//
// It is not stopped on the way out, and that asymmetry is deliberate: other projects depend on it.
func (o Operator) ensureProxy() error {
	containers, err := o.Engine.Containers()
	if err != nil {
		return err
	}

	holder := ""

	for _, container := range containers {
		if !container.Running {
			continue
		}

		if container.Name == proxyContainer {
			return nil
		}

		if holder == "" && holdsAny(container.Published, proxyPorts) {
			holder = container.Name
		}
	}

	if holder != "" {
		return core.Refusal{
			Kind:    "ports_taken",
			Code:    6,
			Message: fmt.Sprintf("'%s' is using port 80 or 443, which the proxy this project needs listens on", holder),
			Hint:    "Stop that environment first, or set USE_PROXY to false here",
		}
	}

	o.announce("Starting the global proxy\n")

	_, err = o.Legacy.Run([]string{"proxy", "up"})

	return err
}

// checkDependenciesAreNotBound refuses an environment whose vendor directory comes from the host.
//
// PHP resolves __DIR__ through the mount, and Composer's generated autoloader builds every path
// from it: with the dependencies bound from outside, the project's own modules are never
// registered and the failure that follows names none of this.
func (o Operator) checkDependenciesAreNotBound(project core.Project) error {
	if o.Platform != "mac" {
		return nil
	}

	containers, err := o.Engine.Containers()
	if err != nil {
		return err
	}

	workdir := o.Workdir
	if workdir == "" {
		workdir = "/var/www/html"
	}

	vendor := path.Join(workdir, project.MagentoDir, "vendor")

	for _, container := range containers {
		if container.Key() != project.Name || container.ComposeService != "phpfpm" {
			continue
		}

		for _, mount := range container.Mounts {
			if mount.Type != "bind" || !overlaps(mount.Destination, vendor) {
				continue
			}

			return core.Refusal{
				Kind:    "vendor_is_bound",
				Code:    1,
				Message: fmt.Sprintf("Vendor cannot be a bind mount, and %s is one", mount.Destination),
				Hint:    o.Binary + " rebuild",
			}
		}
	}

	return nil
}

// Stop stops the environment, optionally saving the database first.
//
// Stopping is an everyday, quick operation, so the copy is asked for rather than taken: a `stop`
// that sometimes takes a minute because it is dumping a database would be an unpleasant surprise.
func (o Operator) Stop(project core.Project, files core.ComposeFiles, services []string, snapshot bool) error {
	if snapshot {
		code, err := o.Legacy.Run([]string{"db", "snapshot"})

		// Not stopping on failure is the point: a stopped environment and no copy, after asking
		// for one, is the worst of the three possible outcomes
		if err != nil || code != 0 {
			return core.Refusal{
				Kind:    "snapshot_failed",
				Code:    1,
				Message: "The snapshot failed, so the environment was left running",
				Hint:    o.Binary + " stop   # to stop without saving",
			}
		}
	}

	return o.Orchestrator.Stop(project, files, services)
}

// Restart is a stop followed by a start, and deliberately not Compose's own `restart`.
//
// Compose's restarts the containers that are there, exactly as they are: a change to the compose
// file is not picked up. Somebody who edits the configuration and runs `restart` expects the
// change to be running afterwards, and the shell implementation gave them that by running its own
// stop and its own start. Being faster at the wrong thing is not an improvement.
func (o Operator) Restart(project core.Project, files core.ComposeFiles, services []string, usesProxy bool) error {
	if err := o.Stop(project, files, services, false); err != nil {
		return err
	}

	return o.Start(project, files, services, false, usesProxy)
}

func (o Operator) announce(message string) {
	if o.Announce != nil {
		o.Announce(message)
	}
}

func holdsAny(published, wanted []string) bool {
	for _, port := range published {
		for _, one := range wanted {
			if port == one {
				return true
			}
		}
	}

	return false
}

// overlaps reports whether two container paths are the same place, or one inside the other.
func overlaps(mount, needle string) bool {
	mount = strings.TrimSuffix(strings.ReplaceAll(mount, "/./", "/"), "/")
	needle = strings.TrimSuffix(strings.ReplaceAll(needle, "/./", "/"), "/")

	return mount == needle ||
		strings.HasPrefix(needle, mount+"/") ||
		strings.HasPrefix(mount, needle+"/")
}
