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
	"io"
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

	// Forced lifts, for this invocation only, the guardrail that refuses from a worktree with no
	// environment of its own anything that would recreate or destroy the main checkout's.
	Forced bool

	// Progress is how an operation long enough to worry about says it is still going, and how it
	// went. It returns the function that ends the step. The command line draws a spinner; an
	// HTTP adapter would send events; a caller that does not care leaves it nil.
	Progress func(label string) func(ok bool, note string)

	// Ask is how a value that cannot be worked out is obtained. The command line reads the
	// terminal; anything else answers however it likes — which is the reason this is not a
	// terminal read buried three layers down.
	Ask func(question, suggestion string) (string, error)
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

	return e.creating(project).Start(project, e.ComposeFiles(project),
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

	return e.creating(project).Restart(project, e.ComposeFiles(project), services, e.UsesProxy(project))
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

//
// The database of a project, which is the second thing after Docker that everything needs: the
// diagnosis reads the domains out of it, an install writes to it, and a developer wants a client.
//

// Query runs one statement against the project's database and writes what it answers.
func (e *Engine) Query(dir, statement string, out io.Writer) (int, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return 0, err
	}

	return e.database().Query(project, statement, out)
}

// Console opens the database client, attached to whatever terminal there is.
func (e *Engine) Console(dir string) (int, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return 0, err
	}

	if _, err := e.database().Ready(project); err != nil {
		return 0, err
	}

	return e.orchestrator(project).Exec(project, e.ComposeFiles(project), "db",
		app.Client(), core.ExecOptions{Interactive: true, Tty: e.attached()})
}

// Feed sends what is on the engine's input into the database client, which is how a dump is
// imported.
func (e *Engine) Feed(dir string) (int, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return 0, err
	}

	if _, err := e.database().Ready(project); err != nil {
		return 0, err
	}

	// Interactive without a terminal: the input is a file or a pipe, and asking for a pseudo
	// terminal there is how a command that works by hand fails in a script
	return e.orchestrator(project).Exec(project, e.ComposeFiles(project), "db",
		app.Client(), core.ExecOptions{Interactive: true})
}

// Import replaces the contents of the project's database with a dump, and does what has to happen
// around that: the DEFINER clauses, the record of the data no longer being anonymised, the
// anonymiser when it is asked for, and pointing the store at this machine.
func (e *Engine) Import(dir string, options app.ImportOptions) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return app.Importer{
		Database:     e.database(),
		Orchestrator: e.orchestrator(project),
		State:        toolinfo.State{Dir: e.options.StateDir},
		Properties:   e.properties(),
		OneOff:       dockerd.OneOff{Out: e.options.Stdout},
		Settings:     filepath.Join(e.options.Root, "data", "local_settings.json"),
		Progress:     e.options.Progress,
		Announce:     e.options.Announce,
		Errors:       e.errors(),
		Binary:       e.options.Binary,
	}.Import(project, e.ComposeFiles(project), options)
}

// Anonymise replaces the personal data of the project's database with data that looks like it.
func (e *Engine) Anonymise(dir string) error {
	project, err := e.Resolve(dir)
	if err != nil {
		return err
	}

	return app.Importer{
		Database: e.database(),
		State:    toolinfo.State{Dir: e.options.StateDir},
		OneOff:   dockerd.OneOff{Out: e.options.Stdout},
		Announce: e.options.Announce,
		Binary:   e.options.Binary,
	}.Anonymise(project)
}

// errors is where something else's complaints go.
func (e *Engine) errors() io.Writer {
	if e.options.Stderr != nil {
		return e.options.Stderr
	}

	return os.Stderr
}

//
// Templates: the data directory of a database, frozen and cloned as files. Seconds instead of the
// tens of minutes an import of the same data costs, which is what makes an environment per branch
// affordable at all.
//

// Templates is every one on this machine, whichever project made it.
func (e *Engine) Templates() ([]core.Template, error) { return e.templates(core.Project{}).List() }

// Freeze copies this project's data directory into a template.
func (e *Engine) Freeze(dir string, options app.FreezeOptions) (core.Template, error) {
	project, configuration, err := e.resolved(dir)
	if err != nil {
		return core.Template{}, err
	}

	return e.templates(project).Freeze(project, e.ComposeFiles(project), configuration, options)
}

// Clone builds this project's data directory from a template.
func (e *Engine) Clone(dir, address string, force bool) (core.Template, error) {
	project, configuration, err := e.resolved(dir)
	if err != nil {
		return core.Template{}, err
	}

	return e.templates(project).Clone(project, configuration, address, force)
}

// Drop deletes a template.
func (e *Engine) Drop(dir, address string, force, interactive bool) (core.Template, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return core.Template{}, err
	}

	return e.templates(project).Drop(project, address, force, interactive)
}

// resolved is the project and the configuration it is built from, which the template operations
// both need — the volume they act on and the image that does the copying come from there and are
// never guessed.
func (e *Engine) resolved(dir string) (core.Project, core.Compose, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return core.Project{}, core.Compose{}, err
	}

	configuration, err := e.Configuration(project)
	if err != nil {
		return core.Project{}, core.Compose{}, err
	}

	return project, configuration, nil
}

//
// Branch environments: a git worktree with an environment of its own.
//

// Worktrees is every branch environment of this project, with what is actually running.
func (e *Engine) Worktrees(dir string) (string, []app.Listed, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return "", nil, err
	}

	parent := e.parentOf(project)

	listed, err := e.worktrees().List(parent)

	return parent, listed, err
}

// RemoveWorktree takes a branch environment away: its containers, its data, its worktree and its
// registration.
func (e *Engine) RemoveWorktree(dir, name string, force, interactive bool) (string, error) {
	project, err := e.Resolve(dir)
	if err != nil {
		return "", err
	}

	// The overlay a branch environment is built from is the parent's registration of it, so the
	// files to take down are that environment's own
	files := e.ComposeFiles(project)
	files.Load = append([]string{files.Load[0], files.Load[1]},
		e.registry().Overlay(e.parentOf(project), name))

	return e.worktrees().Remove(e.parentOf(project), project.Root, name, files, force, interactive)
}

// parentOf is the project a branch environment belongs to, which from the main checkout is simply
// this one.
func (e *Engine) parentOf(project core.Project) string {
	if project.Worktree != nil {
		return project.Worktree.Parent
	}

	return project.Name
}

// AddWorktree gives a branch an environment of its own.
//
// Everything that can be refused is refused before anything is created: half a branch environment
// — a worktree with no registration, a registration with no overlay — is the state nothing else in
// this tool knows how to talk about.
func (e *Engine) AddWorktree(dir string, options app.AddOptions) (app.Added, error) {
	project, configuration, err := e.resolved(dir)
	if err != nil {
		return app.Added{}, err
	}

	worktrees := e.worktrees()

	plan, err := worktrees.Prepare(project, e.UsesProxy(project), options)
	if err != nil {
		return app.Added{}, err
	}

	//
	// Two environments answering on one address is not something the proxy refuses: the routers
	// have different names, so it accepts both and the routing is simply ambiguous.
	//
	if e.addressTaken(plan.Domain) {
		return app.Added{}, core.Refusal{
			Kind: "address_taken", Code: 2,
			Message: "Something is already routed at " + plan.Domain,
			Hint:    e.options.Binary + " proxy status",
		}
	}

	services := make([]string, 0, len(configuration.Services))
	for _, service := range configuration.Services {
		services = append(services, service.Name)
	}

	plan, err = worktrees.Create(project, plan, services, e.Property(project, "WORKDIR_PHP"))
	if err != nil {
		return app.Added{}, err
	}

	e.settle(project, plan, options)

	return app.Added{
		Name: plan.Name, Branch: plan.Branch(), Profile: plan.Profile,
		Path: plan.Path, Project: plan.Project, URL: "https://" + plan.Domain,
	}, nil
}

// settle is everything after the environment exists: the dependencies, the address, the data and
// starting it.
//
// None of it fails the command. By this point the worktree is created and registered, and a
// failure here leaves something somebody can look at and fix — where returning an error would
// leave it created, registered, and reported as not having happened.
func (e *Engine) settle(project core.Project, plan app.Plan, options app.AddOptions) {
	e.shareDependencies(project, plan)

	if !(machine.Host{}).ResolvesLocally(plan.Domain) {
		e.say(plan.Domain + " does not resolve yet\n")
	}

	e.cloneInto(project, plan)

	if !options.Start {
		return
	}

	e.say("Starting " + plan.Project + "...\n")
	e.Start(plan.Path, StartOptions{}) //nolint:errcheck

	e.anonymiseBranch(plan, options)
}

// shareDependencies gives the branch what it needs to run without installing it again.
//
// On macOS the code lives in a named volume, so the volume is copied: nothing on this filesystem
// can be mounted into it. On Linux the main checkout's dependencies are mounted read-only, and
// that decision was already taken when the overlay was written — what is left here is saying so,
// or saying why not.
func (e *Engine) shareDependencies(project core.Project, plan app.Plan) {
	if Platform() == "mac" {
		source := project.Name + "_workspace"

		if !(dockerd.Volumes{}).Exists(source) {
			e.say("The main environment has no code volume yet; this one starts empty\n")

			return
		}

		e.say("Copying the code volume...\n")

		if _, image := configuredDatabase(e, project); image != "" {
			if err := e.templates(project).CopyVolume(source, plan.Project+"_workspace", image); err != nil {
				e.say("The code volume could not be copied\n")
			}
		}

		return
	}

	if plan.SharedVendor {
		e.say("Dependencies are read from the main checkout; nothing was copied.\n")

		return
	}

	//
	// The branch changed its dependencies, so sharing them would be a lie. That is the honest
	// price of having changed them.
	//
	e.say("This branch's composer.lock differs from the main checkout's\n")
	e.say("  Install its dependencies there before using it: cd " + plan.Path +
		" && " + e.options.Binary + " composer install\n")
}

// cloneInto gives the branch its data.
//
// With no template it is not invented: a branch environment sharing the main database would not be
// an isolated environment at all, and a `setup:upgrade` on the branch would land on everybody.
func (e *Engine) cloneInto(project core.Project, plan app.Plan) {
	if !(dockerd.Volumes{}).Exists(core.TemplateVolume(project.Name, "base")) {
		e.say("This project has no database template, so the environment starts empty\n")
		e.say("  Freeze one with " + e.options.Binary + " db freeze\n")

		return
	}

	e.say("Cloning the database...\n")

	if _, err := e.Clone(plan.Path, core.TemplateAddress(project.Name, "base"), true); err != nil {
		e.say("The database could not be cloned; the environment starts empty\n")
	}
}

// anonymiseBranch is why the agent profile exists at all.
//
// An environment called `agent` is one an agent works in, and what an agent reads goes to a model,
// over a network, outside the company. A development database is a copy of production: real names,
// addresses, emails and orders. So this is the moment — the data has just been cloned and nobody
// is waiting on it.
//
// A default and not a rule: reproducing a bug that only happens with one customer's order history
// is a real thing people do, and it is their data and their decision.
func (e *Engine) anonymiseBranch(plan app.Plan, options app.AddOptions) {
	if plan.Profile != "agent" {
		return
	}

	if !options.Anonymise {
		e.say("Not anonymised, as asked. This database holds whatever the original held.\n")

		return
	}

	e.say("Anonymising the branch environment's database...\n")

	if err := e.Anonymise(plan.Path); err != nil {
		// Reported, not swallowed: an environment that was supposed to be anonymised and is not
		// is exactly the situation this feature exists to prevent
		e.say("The database could not be anonymised. It holds whatever the original held; run " +
			e.options.Binary + " masquerade from " + plan.Path + " before letting an agent read it.\n")

		return
	}

	e.say("Anonymised.\n")
}

// addressTaken reports whether the one proxy on this machine is already routing that name.
func (e *Engine) addressTaken(domain string) bool {
	containers, err := (dockerd.Engine{Timeout: e.options.Timeout}).Containers()
	if err != nil {
		return false
	}

	for _, container := range containers {
		if container.Running && container.Routes(domain) {
			return true
		}
	}

	return false
}

func (e *Engine) say(message string) {
	if e.options.Announce != nil {
		e.options.Announce(message)
	}
}

// configuredDatabase is the volume and the image a project's database uses.
func configuredDatabase(e *Engine, project core.Project) (string, string) {
	configuration, err := e.Configuration(project)
	if err != nil {
		return "", ""
	}

	return app.DataDirectory(configuration)
}

func (e *Engine) worktrees() app.Worktrees {
	return app.Worktrees{
		Registry:     e.registry(),
		Engine:       dockerd.Engine{Timeout: e.options.Timeout},
		VCS:          gitvcs.Git{},
		FS:           osfs.FS{},
		Orchestrator: e.orchestrator(core.Project{}),
		Platform:     Platform(),

		// The same lock the shell implementation takes, so that two agents — one of each —
		// creating a branch environment at the same moment take turns instead of both believing
		// they own the name
		Lock: func() (func(), error) {
			lock, err := hmstate.Take("worktrees")
			if err != nil {
				return nil, err
			}

			return lock.Release, nil
		},
		Announce: e.options.Announce,
		Ask:      e.options.Ask,
		Binary:   e.options.Binary,
	}
}

func (e *Engine) templates(project core.Project) app.Templates {
	return app.Templates{
		Volumes:      dockerd.Volumes{},
		OneOff:       dockerd.OneOff{Out: e.options.Stdout},
		Engine:       dockerd.Engine{Timeout: e.options.Timeout},
		Orchestrator: e.orchestrator(project),
		State:        toolinfo.State{Dir: e.options.StateDir},
		Progress:     e.options.Progress,
		Announce:     e.options.Announce,
		Ask:          e.options.Ask,
		Binary:       e.options.Binary,
	}
}

func (e *Engine) database() app.Database {
	return app.Database{
		Engine: dockerd.Engine{Timeout: e.options.Timeout},
		Runner: dockerd.Runner{},
		FS:     osfs.FS{},
		Binary: e.options.Binary,
	}
}

// attached reports whether there is a terminal to give the command.
func (e *Engine) attached() bool {
	in, out := e.options.Stdin, e.options.Stdout
	if in == nil {
		in = os.Stdin
	}

	if out == nil {
		out = os.Stdout
	}

	return terminal(in) && terminal(out)
}

func terminal(file *os.File) bool {
	info, err := file.Stat()

	return err == nil && info.Mode()&os.ModeCharDevice != 0 && info.Mode()&os.ModeDevice != 0
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

	//
	// What the compose files stamp on every container they create, and the reason it is here
	// rather than in whatever called us: an environment created without them looks, to this
	// tool's own inventory, like one made before the labels existed — no project, no worktree,
	// no version. It also changes the configuration hash, so the two implementations would
	// recreate each other's containers.
	//
	environment["COMPOSE_PROJECT_NAME"] = project.Name
	environment["HM_PROJECT"] = project.Name
	environment["HM_ROOT"] = project.Root
	environment["HM_WORKTREE"] = ""
	environment["HM_PROFILE"] = "full"
	environment["HM_AGENT"] = ""

	if project.Worktree != nil {
		environment["HM_WORKTREE"] = project.Worktree.Name
		environment["HM_PROFILE"] = project.Worktree.Profile

		// An environment on the agent profile is an environment an agent works in, and the label
		// is what lets the diagnosis ask about its data
		if project.Worktree.Profile == "agent" {
			environment["HM_AGENT"] = "true"
		}
	}

	return environment
}

// CreationEnvironment is the same plus the two labels that cost something to resolve.
//
// Only for the operations that create containers. `git describe` and reading the Magento version
// out of a composer.lock are about 130 ms together, and paying that on every read would give back
// most of what porting the read commands bought.
func (e *Engine) CreationEnvironment(project core.Project) map[string]string {
	environment := e.Environment(project)

	tooling := toolinfo.Reader{Root: e.options.Root}
	environment["HM_VERSION"] = tooling.Version()
	environment["HM_MAGENTO"] = magentofiles.Reader{}.Version(project.Root, project.MagentoDir)

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

// creating is an operator whose orchestrator stamps the labels a container is created with.
func (e *Engine) creating(project core.Project) app.Operator {
	operator := e.operator(project)
	operator.Orchestrator = e.orchestratorWith(e.CreationEnvironment(project))

	return operator
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
		Forced:       e.options.Forced,
	}
}

func (e *Engine) orchestrator(project core.Project) composelib.Orchestrator {
	return e.orchestratorWith(e.Environment(project))
}

func (e *Engine) orchestratorWith(environment map[string]string) composelib.Orchestrator {
	return composelib.Orchestrator{
		Environment: environment,
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
