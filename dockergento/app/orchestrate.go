package app

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// ProxyStarter starts the global proxy and says whether it had to: Proxy, or a fake of it.
type ProxyStarter interface {
	Up() (bool, error)
}

// Operator runs the everyday operations on an environment: bringing it up, stopping it, looking
// at what it is saying and running something inside it.
type Operator struct {
	Orchestrator ports.Orchestrator
	Engine       ports.ContainerEngine

	// ComposeRunner is the Compose command line itself, for a subcommand this tool does not
	// implement through Orchestrator. Named apart from the Compose method below: Go does not
	// allow a field and a method of the same type to share a name.
	ComposeRunner ports.ComposeRunner

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

	// Ask is how a question with a free-form answer is put — "Stop them all?" and not a menu.
	// Nil means there is nobody to ask, the same as Choose.
	Ask func(question, suggestion string) (string, error)

	// Snapshots takes the copy that `stop` and `down` offer before doing something that cannot be
	// undone.
	Snapshots *Snapshots

	// Proxy is the one router on the machine, started for a project that is routed through it.
	// Nil refuses (proxy_unavailable) when a project needs it started.
	Proxy ProxyStarter

	//
	// What the Linux steps after an environment comes up need (after_start.go).
	//

	// UserPHP and GroupPHP are who PHP runs as inside the containers, from the project's
	// properties: the user and the group whose ids are matched to the host's.
	UserPHP  string
	GroupPHP string

	// RootRunner runs a command as root inside a container and captures what it says, which is
	// how a container's ids are read. Addresses is where the TLS terminator is reached, Files
	// carries this machine's certificate authorities in, and FS is how it is known whether there
	// are any. Nil FS skips the certificates.
	RootRunner ports.ContainerRunner
	Addresses  ports.Addresses
	Files      ports.FileTransfer
	FS         ports.FS

	// Query runs a statement against the project's database, the one `mysql -q` runs, which is
	// how the store's domains are read.
	Query func(project core.Project, statement string, out io.Writer) (int, error)

	// Wait is how the steps give the containers time to spin up, and HostIds is the host user's
	// uid and gid. Both are fields so that a test neither sleeps nor depends on who runs it; nil
	// is time.Sleep and the ids of this process.
	Wait    func(time.Duration)
	HostIds func() (uid, gid int)

	// Errors is where a command run inside a container complains while its output is being read.
	// Nil is this process's own stderr.
	Errors io.Writer

	// Programs is this machine's own programs, which is how `ssl` finds, installs and runs mkcert
	// (certificate.go).
	Programs ports.HostPrograms
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
func (o Operator) Start(project core.Project, files core.ComposeFiles, services []string,
	stopOthers, usesProxy, interactive bool) error {
	if err := o.refuseFromAnUnregisteredWorktree(project, "start"); err != nil {
		return err
	}

	if stopOthers {
		if _, err := o.StopEverything(project, interactive); err != nil {
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

	// What the platform needs once the environment is up: the same steps `post-start` and
	// `rebuild` run, one copy of them for every caller so that they cannot drift
	return o.AfterStart(project, files)
}

// Rebuild stops what was named, or the whole environment, and brings it back up with its images
// built first — what rebuild.sh did with `hm stop` and `up --build -d`.
//
// With nothing named it builds nginx, as the shell did: Compose brings up what nginx depends on
// with it, which is the whole stack. The check that the dependencies are not bound from the host
// runs after every rebuild, services named or not, as rebuild.sh ran validate_bind_mounts.sh.
func (o Operator) Rebuild(project core.Project, files core.ComposeFiles, services []string) error {
	if err := o.refuseFromAnUnregisteredWorktree(project, "rebuild"); err != nil {
		return err
	}

	o.announce("Rebuilding and starting containers in detached mode\n\n")

	if err := o.Orchestrator.Stop(project, files, services); err != nil {
		return err
	}

	building := services
	if len(building) == 0 {
		building = []string{"nginx"}
	}

	if err := o.Orchestrator.UpBuilding(project, files, building); err != nil {
		return err
	}

	if err := o.checkDependenciesAreNotBound(project); err != nil {
		return err
	}

	return o.AfterStart(project, files)
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

	// Nothing stands behind a missing proxy any more (the shell fallback went with bin/run), so an
	// Operator built without one refuses rather than panicking
	if o.Proxy == nil {
		return core.Refusal{
			Kind:    "proxy_unavailable",
			Code:    1,
			Message: "This project is routed through the global proxy, and there is no way to start it from here",
			Hint:    o.Binary + " proxy up",
		}
	}

	o.announce("Starting the global proxy\n")

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
func (o Operator) Stop(project core.Project, files core.ComposeFiles, options core.StopOptions) error {
	if err := o.refuseFromAnUnregisteredWorktree(project, "stop"); err != nil {
		return err
	}

	if options.Snapshot {
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

	return o.Orchestrator.StopWithTimeout(project, files, options.Services, options.Timeout)
}

// Restart is a stop followed by a start, and deliberately not Compose's own `restart`.
//
// Compose's restarts the containers that are there, exactly as they are: a change to the compose
// file is not picked up. Somebody who edits the configuration and runs `restart` expects the
// change to be running afterwards, and the shell implementation gave them that by running its own
// stop and its own start. Being faster at the wrong thing is not an improvement.
func (o Operator) Restart(project core.Project, files core.ComposeFiles, options core.RestartOptions, usesProxy bool) error {
	if err := o.refuseFromAnUnregisteredWorktree(project, "restart"); err != nil {
		return err
	}

	stopping := core.StopOptions{Services: options.Services, Timeout: options.Timeout}
	if err := o.Stop(project, files, stopping); err != nil {
		return err
	}

	return o.Start(project, files, options.Services, false, usesProxy, false)
}

// StopEverything stops every container found running on this machine, machine-wide and with no
// label scoping — what docker-stop-all asks for, and what Start asks for on its own behalf when
// told to stop the rest before coming up.
//
// It does not stop at the first refusal: the rest still stop, and what would not die comes back
// in the error so it can be named.
func (o Operator) StopEverything(project core.Project, interactive bool) (core.MachineStop, error) {
	if err := o.refuseFromAnUnregisteredWorktree(project, "docker-stop-all"); err != nil {
		return core.MachineStop{}, err
	}

	// Not tolerated, unlike the shell's `docker ps -q 2>/dev/null`: a daemon that cannot be
	// reached answered "No containers running" and exit 0, a connection failure read as success.
	containers, err := o.Engine.Containers()
	if err != nil {
		return core.MachineStop{}, err
	}

	ids := make([]string, 0, len(containers))
	others := 0

	for _, item := range containers {
		if !item.Running {
			continue
		}

		ids = append(ids, item.ID)

		if item.ComposeProject != project.Name {
			others++
		}
	}

	total := len(ids)

	if total == 0 {
		o.say("No containers running\n")

		return core.MachineStop{}, nil
	}

	if interactive && o.Ask != nil {
		o.say(fmt.Sprintf("This stops %d container(s) on this machine.\n", total))

		if others > 0 {
			o.say(fmt.Sprintf("%d of them do not belong to '%s'.\n", others, project.Name))
		}

		answer, err := o.Ask("Stop them all? [y/N]:", "")
		if err != nil {
			return core.MachineStop{}, err
		}

		if answer != "y" && answer != "Y" {
			o.say("Nothing was stopped.\n")

			return core.MachineStop{Total: total, Others: others}, nil
		}
	}

	o.say(fmt.Sprintf("Stopping %d container(s)\n", total))

	failed, err := o.Engine.Stop(ids)
	if err != nil {
		return core.MachineStop{}, err
	}

	stopped := total - len(failed)

	if len(failed) > 0 {
		return core.MachineStop{Total: total, Others: others, Stopped: stopped},
			fmt.Errorf("could not stop: %s", strings.Join(failed, ", "))
	}

	return core.MachineStop{Total: total, Others: others, Stopped: stopped}, nil
}

// Compose runs a Compose subcommand this tool does not implement itself, against exactly the
// files and environment given, and hands back that subcommand's own exit code.
//
// No worktree refusal: docker-compose is not in hm_alters_environment, and passing a subcommand
// through is not, by itself, recreating or destroying anything.
func (o Operator) Compose(project core.Project, files []string, environment map[string]string, args []string) (int, error) {
	return o.ComposeRunner.Run(project.Root, files, environment, args)
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
	return RefuseFromAnUnregisteredWorktree(project, command, o.Forced)
}

// RefuseFromAnUnregisteredWorktree is that refusal for a command that is not an Operator's: setup
// creates an environment too, and from such a worktree it would rewrite the committed properties of
// the main checkout's.
func RefuseFromAnUnregisteredWorktree(project core.Project, command string, forced bool) error {
	if !project.InWorktree || project.Worktree != nil || forced {
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

// DownPreview is what Down would remove, and removes nothing: the project's containers, by the
// name Docker gives them, and with Volumes the ones of its volumes that are there. Refused where
// Down would be, so that a preview never lists what the real thing would not touch.
func (o Operator) DownPreview(project core.Project, options core.DownOptions, volumes []string) (core.DownPreview, error) {
	if err := o.refuseFromAnUnregisteredWorktree(project, "down"); err != nil {
		return core.DownPreview{}, err
	}

	containers, err := o.Engine.Containers()
	if err != nil {
		return core.DownPreview{}, err
	}

	preview := core.DownPreview{Containers: []string{}, Volumes: []string{}}

	for _, container := range containers {
		if container.Key() == project.Name {
			preview.Containers = append(preview.Containers, container.Name)
		}
	}

	sort.Strings(preview.Containers)

	if options.Volumes {
		preview.Volumes = append(preview.Volumes, volumes...)
		sort.Strings(preview.Volumes)
	}

	return preview, nil
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
