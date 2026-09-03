package app

import (
	"fmt"
	"path"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
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

	// Forced lifts the guardrail that refuses, from a worktree with no environment of its own,
	// anything that would recreate or destroy the main checkout's.
	Forced bool

	// Choose is how a question with a fixed set of answers is put. Nil means there is nobody to
	// ask, which is not the same as somebody answering: it is the case where the answer has to be
	// what the flags already said.
	Choose func(question string, options []string) (string, error)

	// Snapshots takes the copy that `stop` and `down` offer before doing something that cannot be
	// undone.
	Snapshots *Snapshots

	// Proxy is the one router on the machine, started for a project that is routed through it.
	Proxy *Proxy
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
	if err := o.refuseFromAnUnregisteredWorktree(project, "start"); err != nil {
		return err
	}

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
		if err := o.checkDependenciesAreNotBound(project); err != nil {
			return err
		}
	}

	return o.afterStarting()
}

// afterStarting is what the platform needs once the environment is up.
//
// On macOS, nothing — and it is not asked, because asking costs a shell process on a command
// where that is a fifth of the time. On Linux it matches the container's user and group ids to
// the host's and writes the project's domains into its /etc/hosts, and that is still the shell
// implementation's: the second of the two reads those domains out of the database through a
// command that is not ported, so porting this would mean porting that first.
//
// It is a hand-off and not a fork of the command. Everything else about starting — the proxy, the
// refusals, the error contract — is the same code on both platforms, which is what it costs to
// keep them from drifting.
func (o Operator) afterStarting() error {
	if o.Platform != "linux" {
		return nil
	}

	_, err := o.Legacy.Run([]string{"post-start"})

	return err
}

// ensureProxy starts the one proxy on the machine when this project needs it and it is not up.
//
// It is not stopped on the way out, and that asymmetry is deliberate: other projects depend on it.
func (o Operator) ensureProxy() error {
	containers, err := o.Engine.Containers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		if container.Running && container.Name == proxyContainer {
			return nil
		}
	}

	if holder := holderOf(containers, proxyPorts); holder != "" {
		return core.Refusal{
			Kind:    "ports_taken",
			Code:    6,
			Message: fmt.Sprintf("'%s' is using port 80 or 443, which the proxy this project needs listens on", holder),
			Hint:    "Stop that environment first, or set USE_PROXY to false here",
		}
	}

	o.announce("Starting the global proxy\n")

	if o.Proxy == nil {
		_, err = o.Legacy.Run([]string{"proxy", "up"})

		return err
	}

	_, err = o.Proxy.Up()

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
	if err := o.refuseFromAnUnregisteredWorktree(project, "stop"); err != nil {
		return err
	}

	if snapshot {
		err := o.snapshot(project)

		// Not stopping on failure is the point: a stopped environment and no copy, after asking
		// for one, is the worst of the three possible outcomes
		if err != nil {
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
	if err := o.refuseFromAnUnregisteredWorktree(project, "restart"); err != nil {
		return err
	}

	if err := o.Stop(project, files, services, false); err != nil {
		return err
	}

	return o.Start(project, files, services, false, usesProxy)
}

// refuseFromAnUnregisteredWorktree stops a branch from taking down the environment of the checkout
// it came from.
//
// A worktree with no registration of its own resolves to the main checkout — which is right for
// reading, and is exactly why anything that recreates or destroys an environment must not run
// there. Somebody standing in a branch does not expect `stop` to stop the environment they left
// running in the main one.
//
// Forced is a decision, not a setting: it applies to the one invocation and there is no variable
// and no file that turns the guardrail off for good.
func (o Operator) refuseFromAnUnregisteredWorktree(project core.Project, command string) error {
	if !project.InWorktree || project.Worktree != nil || o.Forced {
		return nil
	}

	return core.Refusal{
		Kind: "blocked_in_worktree",
		Code: 6,
		Message: fmt.Sprintf(
			"'%s' is refused from a worktree: it would recreate or destroy the environment of %s with the mounts of this worktree",
			command, project.Root),
		Hint: fmt.Sprintf("Run it from %s, or repeat with --force", project.Root),
	}
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

// holderOf is the container in the way, asked port by port in the order given.
//
// The order is not a detail: with a full stack up, one container holds 80 and another holds 443,
// and the sentence names one of them. Asking about 80 first is what the shell implementation does,
// and the two have to name the same one or the same situation reads as two different problems.
func holderOf(containers []core.Container, ports []string) string {
	for _, port := range ports {
		for _, container := range containers {
			if !container.Running || container.Name == core.ProxyContainer {
				continue
			}

			if holdsAny(container.Published, []string{port}) {
				return container.Name
			}
		}
	}

	return ""
}

// overlaps reports whether two container paths are the same place, or one inside the other.
func overlaps(mount, needle string) bool {
	mount = strings.TrimSuffix(strings.ReplaceAll(mount, "/./", "/"), "/")
	needle = strings.TrimSuffix(strings.ReplaceAll(needle, "/./", "/"), "/")

	return mount == needle ||
		strings.HasPrefix(needle, mount+"/") ||
		strings.HasPrefix(mount, needle+"/")
}

// Down removes the environment, and answers what happened.
//
// Without its volumes this destroys nothing that cannot be rebuilt, and it is the one-line
// pass-through it always was. With them it deletes the database: one letter of difference, no
// warning and no way back. An environment on this machine was lost exactly that way, which is why
// it says which volumes, offers to save a copy first, and takes no for an answer.
func (o Operator) Down(project core.Project, files core.ComposeFiles, options core.DownOptions,
	volumes []string, interactive bool) (string, error) {
	if err := o.refuseFromAnUnregisteredWorktree(project, "down"); err != nil {
		return "", err
	}

	// Nothing to lose: no volumes going, or none of them there any more, or nobody to ask
	if !options.Volumes || len(volumes) == 0 || !interactive || o.Choose == nil {
		return "destroyed", o.Orchestrator.Down(project, files, options)
	}

	//
	// Three answers, not two. Saving first is offered first because it is the one nobody regrets,
	// and taking the copy automatically would leave a pile of them in the projects that are
	// destroyed on purpose several times a day.
	//
	const (
		saveFirst = "Save a database snapshot, then destroy"
		destroy   = "Destroy without saving"
		cancel    = "Cancel"
	)

	o.say(fmt.Sprintf("\nThis deletes the volumes of '%s', and the database with them:\n\n",
		project.Name))

	for _, volume := range volumes {
		o.say("  " + volume + "\n")
	}

	o.say("\n")

	answer, err := o.Choose("What should happen?", []string{saveFirst, destroy, cancel})
	if err != nil {
		return "", err
	}

	switch answer {
	case saveFirst:
		if o.Snapshots == nil {
			return "", core.Refusal{
				Kind:    "snapshot_failed",
				Code:    1,
				Message: "The snapshot failed, so nothing was destroyed",
				Hint:    o.Binary + " down -v   # to destroy anyway",
			}
		}

		// Not destroying on failure is the point: an environment gone and no copy, after asking
		// for one, is the worst of the three possible outcomes
		if _, err := o.Snapshots.Take(project, "", false); err != nil {
			return "", core.Refusal{
				Kind:    "snapshot_failed",
				Code:    1,
				Message: "The snapshot failed, so nothing was destroyed",
				Hint:    o.Binary + " down -v   # to destroy anyway",
			}
		}

		return "saved", o.Orchestrator.Down(project, files, options)
	case destroy:
		return "destroyed", o.Orchestrator.Down(project, files, options)
	}

	return "", nil
}

// snapshot takes the copy the caller asked for.
//
// Directly rather than through the shell implementation, which is what this did while snapshots
// were still there: one implementation of a copy, and one place where what it writes is decided.
func (o Operator) snapshot(project core.Project) error {
	if o.Snapshots == nil {
		return fmt.Errorf("snapshots are not available here")
	}

	_, err := o.Snapshots.Take(project, "", false)

	return err
}

// say is how this talks to whoever is watching, when there is anybody.
func (o Operator) say(message string) {
	if o.Announce != nil {
		o.Announce(message)
	}
}
