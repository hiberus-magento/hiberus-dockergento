// Package ports declares what the domain needs from the outside world.
//
// Every one of these is implemented by an adapter and by a fake in the tests. That is the whole
// point of the arrangement: resolving a project, reconciling an environment or collecting what is
// abandoned can be exercised without a Docker daemon, a git repository or a filesystem — which is
// exactly the part of the tool that could not be tested before.
package ports

import (
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Properties reads the configuration a project keeps in config/docker/properties.json.
type Properties interface {
	// Load returns the properties of the project rooted at dir. A project with no properties
	// file is not an error: it is a directory that is not a project, and the caller decides
	// what that means.
	Load(dir string) (map[string]string, error)

	// Set records one property, leaving the rest of the file alone. The file is committed, so
	// rebuilding it from the keys somebody happened to be thinking about is how a project loses
	// a setting nobody was looking at.
	Set(dir, key, value string) error
}

// VCS answers the questions this tool asks git.
type VCS interface {
	// Resolve reports the main checkout of the repository containing dir, and whether dir is a
	// linked worktree of it. A directory outside a repository is not an error either.
	Resolve(dir string) (mainRoot string, isWorktree bool, worktreeName string, err error)

	// Tracked is the repository's own top-level entries, which is what decides what has to be
	// mounted into a container that does not see the checkout.
	Tracked(dir string) ([]string, error)

	// Dirty reports whether a working directory has changes nobody has committed. Containers and
	// databases can be rebuilt in seconds; uncommitted code cannot be rebuilt at all.
	Dirty(dir string) bool

	// RemoveWorktree takes a linked worktree away, and Prune clears the administrative entry a
	// directory somebody deleted by hand leaves behind — git refuses to reuse the name until it
	// is gone.
	RemoveWorktree(root, path string, force bool) error
	Prune(root string) error

	// AddWorktree creates a linked worktree, on an existing branch or on a new one, and gives
	// back what git said when it could not.
	AddWorktree(root, path, branch string) (string, error)
}

// Changes answers what a branch changed, for the checks that look only at that.
type Changes interface {
	// Exists reports whether a reference names something in the repository at dir.
	Exists(dir, reference string) bool

	// ChangedSince is the files the branch added, copied, modified or renamed since it left base,
	// matching pathspec. Nothing, when git cannot say.
	ChangedSince(dir, base, pathspec string) []string
}

// Registry is where branch environments are recorded, outside the checkout: properties.json is a
// committed file, and a worktree's project name written there would travel in somebody's commit.
type Registry interface {
	// Worktree returns the registration of a branch environment, or nil when there is none.
	// A worktree with no registration is the case the guardrails refuse.
	Worktree(parent, name string) (*core.Worktree, error)

	// Worktrees is every branch environment of a project.
	Worktrees(parent string) ([]core.Worktree, error)

	// Forget removes a registration and the overlay beside it.
	Forget(parent, name string) error

	// Overlay is the compose file that carries a branch environment's profile and routing, and
	// WriteOverlay writes it.
	Overlay(parent, name string) string
	WriteOverlay(parent, name, contents string) error

	// Save records a branch environment.
	Save(parent string, worktree core.Worktree) error

	// Parents is every project that has registrations, which is how what is abandoned is found
	// without being told where to look.
	Parents() ([]string, error)
}

// ContainerEngine is the Docker daemon.
type ContainerEngine interface {
	// Containers returns every container on the machine, running or not. One call: the inventory
	// is built by grouping them, and asking per environment cost seconds on a machine with a
	// hundred of them.
	Containers() ([]core.Container, error)

	// Remove takes containers away by id, forcing them: what is being removed has no directory
	// left, so there is no configuration to stop it politely with.
	Remove(ids []string) error

	// Stop stops containers by id, without removing them, and does not stop at the first refusal:
	// like `docker stop a b c`, it asks all of them and reports which did not die, so a container
	// that will not stop does not spare the rest. err is only for never reaching the daemon at
	// all.
	Stop(ids []string) (failed []string, err error)
}

// Daemon is what the diagnosis asks Docker beyond the container list.
//
// Separate from ContainerEngine because they are asked for different reasons: the inventory needs
// containers, and only the diagnosis needs to know how much memory the VM was given or whether an
// image can still be pulled.
type Daemon interface {
	// Reachable is the first question, and the one that makes every other answer meaningless
	// when it is no.
	Reachable() bool

	// Info is what the daemon says about itself.
	Info() (core.DaemonInfo, error)

	// Leftovers counts volumes and dangling images. Counting them is 0.13s where computing their
	// real sizes was 18s on a machine with 152 volumes, and it catches the same thing: an
	// environment graveyard nobody cleans up.
	Leftovers() (volumes, danglingImages int, err error)

	// ImageAvailability reports whether an image is already here and whether it could be pulled.
	ImageAvailability(image string) (local, pullable bool)
}

// Machine is what the host says about itself, as opposed to what Docker says.
type Machine interface {
	// MemoryBytes is the machine's own memory, which on macOS is not the containers'.
	MemoryBytes() int64

	// FreeDiskGB is the space left on the startup disk, and whether it could be read at all.
	FreeDiskGB() (int, bool)

	// Listening is every port held on this machine, with the process holding it when the tool
	// that listed them names it.
	Listening() ([]core.Listener, error)

	// MarkedHosts are the entries this tool put in /etc/hosts, by the domain each one names.
	MarkedHosts(marker string) []string

	// InGroup answers whether the user belongs to a group, which on Linux decides whether they
	// can talk to Docker at all.
	InGroup(name string) bool

	// Mkcert reports whether it is installed and where its local authority lives.
	Mkcert() (installed bool, caroot string)

	// HostsEntry is whether /etc/hosts sends this domain anywhere.
	HostsEntry(domain string) bool

	// HostsMalformed is whether /etc/hosts has the entry this tool wrote until 1.5.0 for this
	// domain, which resolves it to 0.0.0.0 and therefore nowhere Firefox will follow.
	HostsMalformed(domain string) bool

	// ResolvesLocally is whether the name resolves to a loopback address. Resolving to something
	// else is not enough: a domain that answers with a real internet address belongs to somebody.
	ResolvesLocally(domain string) bool
}

// FS is the little the domain needs to know about the filesystem.
type FS interface {
	// IsDir reports whether the path is a directory that exists. It is how an environment whose
	// project was deleted is told from one that is merely stopped.
	IsDir(path string) bool

	// Exists reports whether the path is there at all.
	Exists(path string) bool

	// Read returns a file's contents, empty when there are none to read.
	Read(path string) string

	// MkdirAll makes a directory and whatever is missing above it.
	MkdirAll(path string) error
}

// Branches reports the branch checked out in a working directory.
type Branches interface {
	Branch(dir string) string
}

// ComposeConfig reads a project's Compose configuration.
type ComposeConfig interface {
	Load(root, name string, files []string) (core.Compose, error)
}

// MagentoFiles reads what the project's own files say, which works with the environment stopped —
// and that is the point: the first question anybody asks is asked before anything is running.
type MagentoFiles interface {
	Version(root, magentoDir string) string
	Mode(root, magentoDir string) string
	AdminPath(root, magentoDir string) string
}

// Tooling is what the machine says about itself.
type Tooling interface {
	Version() string
	ComposeVersion() string
	Workdir() string
	Xdebug(project string) string
}

// DataState is whether this copy of the data has been anonymised, and when.
type DataState interface {
	Anonymisation(environment string) (string, string)

	// Record and Clear are the two halves that keep it honest. Everything that replaces the
	// contents of a database clears it: whatever the new contents are, nobody anonymised them.
	Record(environment, at string) error
	Clear(environment string) error
}

// Orchestrator brings environments up and down, and runs things inside them.
//
// It is Compose, called as a library rather than as a command. The same code the `docker compose`
// CLI runs, in this process: it computes the same configuration hash and stamps the same labels,
// so what it creates and what the command creates are the same containers — which is the property
// everything else here depends on, and the one that is tested rather than assumed.
type Orchestrator interface {
	// Up creates and starts what is missing and leaves alone what already matches.
	Up(project core.Project, files core.ComposeFiles, services []string) error

	// UpBuilding is Up with the images of the services that have a build section built first,
	// whether or not they are already there: what `up --build` does, and what `rebuild` asks for.
	UpBuilding(project core.Project, files core.ComposeFiles, services []string) error

	// Down removes an environment, doing what Compose's own `down` does and no more: what is
	// removed beyond the containers and the networks is what the caller asked for.
	Down(project core.Project, files core.ComposeFiles, options core.DownOptions) error

	// Stop stops without removing: an everyday operation that has to be quick and keep the data.
	// It stops options.Services (empty is all of them), giving each container options.Timeout
	// seconds before it is killed, nil being Compose's own default. options.Snapshot is not its
	// business: the copy is taken, or not, before it is asked.
	Stop(project core.Project, files core.ComposeFiles, options core.StopOptions) error

	// Logs writes the logs of the services named, or of all of them, until it is interrupted or
	// they end.
	Logs(project core.Project, files core.ComposeFiles, services []string, options core.LogOptions) error

	// Exec runs a command inside a running service and returns its exit code, which is the
	// command's own — a wrapper that flattened it would break everything that branches on it.
	Exec(project core.Project, files core.ComposeFiles, service string, command []string, options core.ExecOptions) (int, error)
}

// ContainerRunner runs a command inside a container that is already running, and gives back what
// it said.
//
// Separate from the orchestrator's Exec, which attaches the terminal: this one captures. Both are
// needed and they are not the same thing — an interactive client wants the terminal, and a query
// whose answer somebody has to read wants the bytes.
type ContainerRunner interface {
	Run(container string, command []string, environment []string, out io.Writer) (int, error)

	// Feed sends a stream into the command's input, which is how a dump the size of a Magento
	// database gets in without ever existing twice on disk.
	Feed(container string, command []string, in io.Reader, out io.Writer) (int, error)

	// Capture is the same as Run for a command whose output is being kept rather than read: the
	// two streams are separated, and there is no deadline.
	//
	// Both differences are the same case. A dump is the output, so a warning written to the
	// error stream would end up inside the copy — and a copy with a sentence in the middle of it
	// is not a copy. And it takes as long as it takes: a deadline that made sense for a query
	// would cut a real database's dump off in the middle and leave a file that looks finished.
	Capture(container string, command []string, out, errors io.Writer) (int, error)
}

// OneOff runs a container that is not part of any environment and removes it afterwards.
//
// The anonymiser is a tool in its own image, attached to the network the database is on. It is
// not a service of the project and it must not become one: a container in the compose file is a
// container somebody has to remember to remove.
type OneOff interface {
	Run(image string, command []string, network string, mounts []core.Bind, tty bool) (int, error)

	// Volumes is the same thing with volumes attached instead of directories of this machine,
	// and its output captured. It is how a data directory is measured and copied: on macOS the
	// volume lives in a virtual machine and its mountpoint does not exist on this filesystem, so
	// there is nothing to look at from outside.
	Volumes(image string, command []string, mounts map[string]string, out io.Writer) (int, error)
}

// Relays are containers that belong to no environment and are found by name: what holds a
// public tunnel or a forwarded port open for as long as somebody uses it.
type Relays interface {
	// Remove takes away, forcing them, the containers named name — or every one whose name starts
	// with it, when prefix — and reports whether there was any.
	Remove(name string, prefix bool) (bool, error)

	// Start creates and starts one, detached, pulling its image first when it is not here.
	Start(relay core.Relay) error

	// Logs is everything it has written so far, both streams together.
	Logs(name string) (string, error)

	// Wait returns when it stops, which is when somebody closed it.
	Wait(name string) error
}

// ServiceLister answers which services a project's Compose files define, running or not — what
// `docker compose config --services` prints.
type ServiceLister interface {
	Services(project core.Project, files core.ComposeFiles) ([]string, error)
}

// Exposure answers which ports a container exposes, from its image or its configuration, whether
// or not they are published — what `docker inspect -f '{{.Config.ExposedPorts}}'` lists.
type Exposure interface {
	Exposed(container string) ([]int, error)
}

// VolumeStore is the Docker volumes on this machine.
type VolumeStore interface {
	// Labelled returns every volume carrying a label, with the labels it carries.
	Labelled(label string) ([]map[string]string, error)

	// Exists reports whether a volume is there.
	Exists(name string) bool

	// Labels of one volume.
	Labels(name string) map[string]string

	// Create makes one with the labels given, and Remove takes it away.
	Create(name string, labels map[string]string) error
	Remove(name string) error

	// All is every volume on this machine, whether this tool made it or not: a volume carries no
	// hm labels, so the only way to attribute one is through the containers of its project.
	All() ([]string, error)

	// Users are the containers holding a volume, of any state. A stopped container still holds
	// one, which is why removing it is refused, and saying so beforehand is better than relaying
	// Docker's own wording.
	Users(name string) ([]string, error)
}

// Addresses answers where a container can be reached on the networks it is attached to.
//
// The self-routing entries a Linux environment writes into its php container point at the TLS
// terminator by this address, which is what lets the store call itself by its own domain.
type Addresses interface {
	// Address is the container's address on every network it is attached to, written one after
	// the other in the order of the networks' names — what
	// `docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}'` prints.
	Address(container string) (string, error)
}

// NetworkStore is the shared network the proxy routes through.
type NetworkStore interface {
	Exists(name string) bool
	Ensure(name string) error
}

// ComposeRunner is the Compose command line itself, for the subcommands this tool does not
// implement. Separate from Orchestrator, which performs operations through the library: this one
// hands an arbitrary subcommand to the binary and answers with the code it exited with.
type ComposeRunner interface {
	// Run executes a Compose subcommand from dir, against these files and this environment, wired
	// to this process's terminal.
	Run(dir string, files []string, environment map[string]string, args []string) (int, error)
}

// HostPrograms finds and runs programs on this machine, outside every container.
//
// It is what `ssl` needs from the host: whether mkcert is there, installing it when it is not, and
// running it. A port so that a test can say what each program answers without brew, apt, sudo,
// curl or mkcert ever running.
type HostPrograms interface {
	// LookPath is where a program is on the PATH, or an error when it is not there.
	LookPath(name string) (string, error)

	// Run executes a program in dir, attached to this process's terminal, and fails when it
	// cannot be started or exits with anything but 0.
	Run(dir, name string, args ...string) error
}

// Legacy runs a command of the shell implementation.
//
// It exists because the migration is a strangler and not a rewrite: what has not been ported yet
// still runs, and a project can add commands of its own in config/hm/commands, which will always
// be shell. This port does not go away when the migration ends.
type Legacy interface {
	// Run executes the shell CLI with these arguments, wired to the current terminal, and
	// returns its exit code.
	Run(args []string) (int, error)
}

// FileTransfer moves files between this machine and a container.
//
// It exists for the case macOS makes necessary: the code lives in a volume for speed, so what is
// written on one side has to be carried to the other. It is also how a certificate gets into the
// container that serves it.
type FileTransfer interface {
	Into(container, source, target string) error
	From(container, source, target string) error
}

// CopyMeter receives how far one copy has got: the bytes and the files carried so far. It is
// called from inside the copy, as often as every block, so it has to be cheap and must not block.
type CopyMeter func(bytes, files int64)

// CopyWatch shows one copy while it happens. It is given the bytes the copy is expected to carry,
// or 0 when nobody knows, and returns what receives the counts and what ends the copy's display,
// told whether the copy failed, so that a broken copy is never shown as a finished one.
type CopyWatch func(total int64) (meter CopyMeter, end func(failed bool))

// ReleaseWatch shows the move to another release of the tool while it happens: looking the release
// up, downloading it, checking it and putting it in place. The zero value watches nothing, which is
// what a caller with nobody looking at a terminal hands.
type ReleaseWatch struct {
	// Step shows a step running under label and returns what ends it: done is the line the step
	// leaves behind, "" for none, and failed says it did not finish.
	Step func(label string) (end func(done string, failed bool))

	// Download shows the download of the binary, its total the bytes the server announced.
	Download CopyWatch
}

// MeteredFileTransfer is a FileTransfer that can also say how far a copy has got as it goes.
type MeteredFileTransfer interface {
	IntoMetered(container, source, target string, meter CopyMeter) error
	FromMetered(container, source, target string, meter CopyMeter) error
}

// Resolver answers whether a name already reaches this machine.
//
// Asked of the machine's own resolution rather than of a file: a wildcard resolver for the TLD
// makes an entry in the hosts file pointless, and that entry is what costs a password prompt per
// project and stays for as long as the machine lives.
type Resolver interface {
	ResolvesLocally(domain string) bool
}
