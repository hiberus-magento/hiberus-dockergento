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

	// Dirty reports whether a working directory has changes nobody has committed. Containers and
	// databases can be rebuilt in seconds; uncommitted code cannot be rebuilt at all.
	Dirty(dir string) bool

	// RemoveWorktree takes a linked worktree away, and Prune clears the administrative entry a
	// directory somebody deleted by hand leaves behind — git refuses to reuse the name until it
	// is gone.
	RemoveWorktree(root, path string, force bool) error
	Prune(root string) error
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

	// Overlay is the compose file that carries a branch environment's profile and routing.
	Overlay(parent, name string) string
}

// ContainerEngine is the Docker daemon.
type ContainerEngine interface {
	// Containers returns every container on the machine, running or not. One call: the inventory
	// is built by grouping them, and asking per environment cost seconds on a machine with a
	// hundred of them.
	Containers() ([]core.Container, error)
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

	// InGroup answers whether the user belongs to a group, which on Linux decides whether they
	// can talk to Docker at all.
	InGroup(name string) bool

	// Mkcert reports whether it is installed and where its local authority lives.
	Mkcert() (installed bool, caroot string)

	// HostsEntry is whether /etc/hosts sends this domain anywhere.
	HostsEntry(domain string) bool

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

	// Down removes an environment. With volumes, its data goes too — which is the whole point
	// when the environment being removed is a branch's.
	Down(project core.Project, files core.ComposeFiles, volumes bool) error

	// Stop stops without removing: an everyday operation that has to be quick and keep the data.
	Stop(project core.Project, files core.ComposeFiles, services []string) error

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

	// Users are the containers holding a volume, of any state. A stopped container still holds
	// one, which is why removing it is refused, and saying so beforehand is better than relaying
	// Docker's own wording.
	Users(name string) ([]string, error)
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
