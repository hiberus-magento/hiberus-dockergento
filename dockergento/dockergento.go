// Package dockergento is the tool as a library.
//
// Everything `hm` can answer or do is here, without a terminal anywhere in it: what a project is,
// what is running on this machine, what is wrong with it, and how to bring it up or down. The
// command line is one way in and not the only one — an HTTP adapter for the web interface and an
// MCP one for agents are the same calls with a different shape around them.
//
// It is a package and not a second binary on purpose. Bringing an environment up in another
// process means paying for the process and for serialising the answer, which is what the shell
// implementation did and what measuring it ended: `describe` costs 94 ms in here and could not
// cost less than 130 through a boundary. The seam that matters is between the domain and the
// outside world, and that one is the ports, not a socket.
package dockergento

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/composecfg"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/composelib"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/dockerd"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/fsprops"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/hmstate"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/legacy"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/machine"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/magentofiles"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/osfs"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/toolinfo"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Options is what an engine needs to know about where it is running.
//
// All of it optional, and all of it something a caller that is not the CLI would get wrong by
// default: the tool's own directory is worked out from the running executable, which is right for
// `hm` and is the web server's binary for anything else.
type Options struct {
	// Root is the tool's own installation. Its data/properties.json holds the defaults every
	// project's own file is merged over, and its compose template is what answers "which ports
	// would an environment need here" when there is no project to ask.
	Root string

	// StateDir is where this machine's copy of the data is recorded — whether it has been
	// anonymised, and when. Empty means ~/.hm/state.
	StateDir string

	// WorktreeDir is where branch environments are registered, outside any checkout. Empty means
	// ~/.hm/worktrees.
	WorktreeDir string

	// ShellRoot is where the shell implementation lives, for the commands that are still its. It
	// does not go away when the migration ends: a project can add commands of its own.
	ShellRoot string

	// Binary is the name the tool is invoked as, because every hint an answer carries is a
	// command the reader is meant to be able to paste. Empty means "hm".
	Binary string

	// Timeout bounds every call to the daemon. A daemon that is starting answers eventually or
	// never, and something that hangs is worse than something that fails.
	Timeout time.Duration

	// Stdin, Stdout and Stderr are for the operations that stream rather than answer: logs, exec,
	// and the progress of bringing an environment up. Empty means the process's own.
	Stdin, Stdout, Stderr *os.File

	// Announce is how the steps that take a while say what they are doing. The CLI paints them;
	// an HTTP adapter would send them; a caller that does not care leaves it nil.
	Announce func(string)
}

// Engine is the tool.
type Engine struct {
	options Options
}

// New builds an engine. The zero Options are the right ones for a program installed beside the
// shell tree, which is what `hm` is.
func New(options Options) *Engine {
	if options.Root == "" {
		options.Root = installedRoot()
	}

	if options.Binary == "" {
		options.Binary = "hm"
	}

	if options.Timeout == 0 {
		options.Timeout = 10 * time.Second
	}

	return &Engine{options: options}
}

// Resolve answers the first question every command asks: what project is this directory in?
//
// A directory that is not a project is not an error — several things are legitimately run outside
// one — so the answer is a project with no name, and the caller decides what that means.
func (e *Engine) Resolve(dir string) (core.Project, error) {
	return app.Resolver{
		Properties: e.properties(),
		VCS:        gitvcs.Git{},
		Registry:   e.registry(),
	}.Resolve(dir)
}

// Environments is every environment on this machine, whether or not its project is still there.
func (e *Engine) Environments() ([]core.Environment, error) {
	return app.Inventory{
		Engine:   dockerd.Engine{Timeout: e.options.Timeout},
		FS:       osfs.FS{},
		Branches: osfs.Branches{},
	}.Environments()
}

// Describe is everything that defines a project: versions, services, addresses and state.
//
// The credentials are only there when they are asked for. An answer that carried the database
// password by default would end up in logs, in issues and in an agent's context.
func (e *Engine) Describe(dir string, withSecrets bool) (core.Description, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return core.Description{}, err
	}

	return app.Describer{
		Engine:  dockerd.Engine{Timeout: e.options.Timeout},
		Compose: composecfg.Loader{Environment: e.Environment(project)},
		Magento: magentofiles.Reader{},
		Tooling: toolinfo.Reader{Root: e.options.Root, WorkdirPHP: e.Property(project, "WORKDIR_PHP")},
		State:   toolinfo.State{Dir: e.options.StateDir},
		Machine: Platform(),
	}.Describe(project, e.ComposeFiles(project), withSecrets)
}

// Diagnose is what has to be true for an environment to work, asked all at once.
//
// It answers outside a project too, about the machine, which is the question somebody has when
// nothing works anywhere. Only names a single check when that is all that is wanted.
func (e *Engine) Diagnose(dir, only string) (core.Diagnosis, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return core.Diagnosis{}, err
	}

	inProject := project.Name != "" &&
		(osfs.FS{}).Exists(filepath.Join(project.Root, "config", "docker", "properties.json"))

	return app.Doctor{
		Daemon:  dockerd.Daemon{Timeout: e.options.Timeout},
		Engine:  dockerd.Engine{Timeout: e.options.Timeout},
		Compose: composecfg.Loader{Environment: e.Environment(project)},
		Magento: magentofiles.Reader{},
		Tooling: toolinfo.Reader{Root: e.options.Root},
		State:   toolinfo.State{Dir: e.options.StateDir},
		Machine: machine.Host{},
		FS:      osfs.FS{},

		Project:      project,
		InProject:    inProject,
		ComposeFiles: e.ComposeFiles(project),
		Template:     filepath.Join(e.options.Root, "docker-compose", "docker-compose.template.yml"),
		Platform:     Platform(),
		Binary:       e.options.Binary,
		Profile:      os.Getenv("HM_PROFILE"),
		Agent:        os.Getenv("HM_AGENT"),
	}.Diagnose(only), nil
}

// StartOptions is what can be asked for around bringing an environment up.
type StartOptions struct {
	// Services names what to start. Empty is the whole environment, and only then are the checks
	// that are about the environment as a whole run.
	Services []string

	// StopOthers stops every other environment first, for a machine that cannot hold two.
	StopOthers bool
}

// Start brings the environment up, and does the things around it nobody should have to remember:
// the proxy a project needs, and the check that its dependencies are not bound from the host.
func (e *Engine) Start(dir string, options StartOptions) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return e.operator(project).Start(project, e.ComposeFiles(project),
		options.Services, options.StopOthers, e.UsesProxy(project))
}

// Stop stops without removing, and only takes a copy of the database when asked. A stop that
// sometimes took a minute because it was dumping a database would be an unpleasant surprise.
func (e *Engine) Stop(dir string, services []string, snapshot bool) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return e.operator(project).Stop(project, e.ComposeFiles(project), services, snapshot)
}

// Restart is a stop and a start, so that a change to the configuration is running afterwards.
func (e *Engine) Restart(dir string, services []string) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return e.operator(project).Restart(project, e.ComposeFiles(project), services, e.UsesProxy(project))
}

// Logs writes what the services are saying to the engine's output, until it is interrupted or
// they end.
func (e *Engine) Logs(dir string, services []string, options core.LogOptions) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return e.orchestrator(project).Logs(project, e.ComposeFiles(project), services, options)
}

// Exec runs a command inside a service and returns that command's own exit code — flattening it
// would break everything that branches on it.
func (e *Engine) Exec(dir, service string, command []string, options core.ExecOptions) (int, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return 0, err
	}

	return e.orchestrator(project).Exec(project, e.ComposeFiles(project), service, command, options)
}

// Configuration is a project's resolved Compose configuration: what the files say after being
// merged, interpolated and validated.
func (e *Engine) Configuration(project core.Project) (core.Compose, error) {
	return composecfg.Loader{Environment: e.Environment(project)}.
		Load(project.Root, project.Name, e.ComposeFiles(project).Load)
}

// Shell runs a command of the shell implementation, for what is not ported and for the commands a
// project adds of its own.
func (e *Engine) Shell(args []string) (int, error) {
	return legacy.Runner{Root: e.options.ShellRoot}.Run(args)
}

// ComposeFiles is what a project is built from.
//
// Two files the project declares — its own and the overlay for this platform — and up to one more
// that it does not:
//
//   - a project routed through the global proxy carries a third file that removes its published
//     ports and adds the routing. It is generated by `setup`, so its absence just means this
//     project does not use the proxy.
//   - a branch environment carries an overlay that lives outside the checkout, next to its
//     registration, and carries its profile and its address. It is loaded instead of the proxy's,
//     never as well: the proxy overlay claims the main environment's address.
//
// Loading the wrong list is not a cosmetic mistake. Without the proxy overlay a proxied project
// reads as publishing ports it does not publish; without the worktree overlay a branch
// environment reads as the main one.
func (e *Engine) ComposeFiles(project core.Project) core.ComposeFiles {
	values := e.Properties(project)

	base := values["DOCKER_COMPOSE_FILE"]
	if base == "" {
		base = "docker-compose.yml"
	}

	overlay := values["DOCKER_COMPOSE_FILE_LINUX"]
	if Platform() == "mac" {
		overlay = values["DOCKER_COMPOSE_FILE_MAC"]
	}

	if overlay == "" {
		overlay = "docker-compose.dev." + Platform() + ".yml"
	}

	files := core.ComposeFiles{
		Load:     []string{base, overlay},
		Declared: []string{base, overlay},
	}

	if project.Worktree != nil {
		files.Load = append(files.Load, e.registry().Overlay(project.Worktree.Parent, project.Worktree.Name))

		return files
	}

	proxy := strings.TrimSuffix(base, ".yml") + ".proxy.yml"
	if (osfs.FS{}).Exists(filepath.Join(project.Root, proxy)) {
		files.Load = append(files.Load, proxy)
	}

	return files
}

// Properties are the project's, merged over the ones the tool ships — which is why a project that
// never set WORKDIR_PHP still has one.
func (e *Engine) Properties(project core.Project) map[string]string {
	values, err := e.properties().Load(project.Root)
	if err != nil {
		return map[string]string{}
	}

	return values
}

// Property is one of them.
func (e *Engine) Property(project core.Project, key string) string {
	return e.Properties(project)[key]
}

// Environment is what the variables inside the compose files interpolate against: the process's
// environment with the project's properties on top, which is what `set -a` did in the shell
// implementation.
func (e *Engine) Environment(project core.Project) map[string]string {
	environment := map[string]string{}

	for _, entry := range os.Environ() {
		if at := strings.IndexByte(entry, '='); at > 0 {
			environment[entry[:at]] = entry[at+1:]
		}
	}

	for key, value := range e.Properties(project) {
		environment[key] = value
	}

	environment["COMPOSE_PROJECT_NAME"] = project.Name
	environment["HM_ROOT"] = project.Root

	return environment
}

// UsesProxy reports whether this project is routed through the one proxy on the machine.
//
// Never for a branch environment: it does not carry the proxy overlay — that overlay claims the
// main environment's address — so starting the proxy for it would achieve nothing, and could
// refuse the start outright when another environment happens to hold port 80.
func (e *Engine) UsesProxy(project core.Project) bool {
	if project.Worktree != nil {
		return false
	}

	switch strings.ToLower(e.Property(project, "USE_PROXY")) {
	case "true", "yes", "1":
		return true
	}

	return false
}

// Platform is "mac" or "linux", which is the name this tool has always used for them.
func Platform() string {
	if runtime.GOOS == "darwin" {
		return "mac"
	}

	return runtime.GOOS
}

func (e *Engine) operator(project core.Project) app.Operator {
	return app.Operator{
		Orchestrator: e.orchestrator(project),
		Engine:       dockerd.Engine{Timeout: e.options.Timeout},
		Legacy:       legacy.Runner{Root: e.options.ShellRoot},
		Announce:     e.options.Announce,
		Platform:     runtime.GOOS,
		Binary:       e.options.Binary,
		Workdir:      e.Property(project, "WORKDIR_PHP"),
	}
}

func (e *Engine) orchestrator(project core.Project) composelib.Orchestrator {
	return composelib.Orchestrator{
		Environment: e.Environment(project),
		Stdin:       e.options.Stdin,
		Stdout:      e.options.Stdout,
		Stderr:      e.options.Stderr,
	}
}

func (e *Engine) properties() fsprops.Reader {
	if e.options.Root == "" {
		return fsprops.Reader{}
	}

	return fsprops.Reader{Defaults: filepath.Join(e.options.Root, "data", "properties.json")}
}

func (e *Engine) registry() hmstate.Registry {
	return hmstate.Registry{Dir: e.options.WorktreeDir}
}

// installedRoot is the tool's own directory, worked out from the running executable.
//
// Right for `hm`, which lives in bin/ of the tree it belongs to, and wrong for anything else —
// which is why it is only the default and Options.Root exists.
func installedRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	return filepath.Dir(filepath.Dir(executable))
}
