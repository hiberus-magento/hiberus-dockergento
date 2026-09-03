package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Branch environments: a git worktree with an environment of its own.
//
// A worktree with no registration keeps the guardrails that stop it from recreating the main
// checkout's environment with its own mounts. One with a registration has its own containers, its
// own database and its own address, and the difference between the two is a file outside the
// checkout — because config/docker/properties.json is committed, and a worktree's project name
// written there would travel in somebody's commit.
type Worktrees struct {
	Registry     ports.Registry
	Engine       ports.ContainerEngine
	VCS          ports.VCS
	FS           ports.FS
	Orchestrator ports.Orchestrator

	// Platform is "mac" or "linux": on macOS the code lives in a named volume and the
	// dependencies cannot be mounted from this filesystem at all.
	Platform string

	// Lock is how two agents creating a branch environment at the same moment take turns. It
	// returns the function that gives it back.
	Lock func() (func(), error)

	Announce func(string)
	Ask      func(question, suggestion string) (string, error)

	Binary string
}

// Listed is a branch environment as somebody reads it.
type Listed struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Profile string `json:"profile"`
	State   string `json:"state"`
	URL     string `json:"url"`
	Path    string `json:"path"`
}

// List is every branch environment of a project, with what is actually running.
//
// The state comes from the containers and the directory, not from the registration: a registry
// that answered "running" for something somebody stopped by hand would be worse than not asking.
func (w Worktrees) List(parent string) ([]Listed, error) {
	registered, err := w.Registry.Worktrees(parent)
	if err != nil {
		return nil, err
	}

	containers, err := w.Engine.Containers()
	if err != nil {
		return nil, err
	}

	running := map[string]bool{}

	for _, container := range containers {
		if container.Running {
			running[container.ComposeProject] = true
		}
	}

	listed := make([]Listed, 0, len(registered))

	for _, worktree := range registered {
		state := "stopped"

		if running[worktree.Project] {
			state = "running"
		}

		// A directory somebody deleted by hand leaves a registration behind, and saying "stopped"
		// about it would send them looking for containers that are not the problem
		if !w.FS.IsDir(worktree.Path) {
			state = "missing"
		}

		listed = append(listed, Listed{
			Name: worktree.Name, Branch: worktree.Branch, Profile: worktree.Profile,
			State: state, URL: "https://" + worktree.Domain, Path: worktree.Path,
		})
	}

	return listed, nil
}

// Remove takes a branch environment away: its containers, its data, its worktree and its
// registration.
func (w Worktrees) Remove(parent, root, name string, files core.ComposeFiles,
	force, interactive bool) (string, error) {
	if name == "" {
		return "", core.Refusal{
			Kind: "missing_name", Code: 2,
			Message: "Which branch environment should be removed?",
			Hint:    w.Binary + " worktree list",
		}
	}

	worktree, err := w.Registry.Worktree(parent, name)
	if err != nil {
		return "", err
	}

	if worktree == nil {
		return "", core.Refusal{
			Kind: "unknown_worktree", Code: 2,
			Message: fmt.Sprintf("'%s' is not a branch environment of this project", name),
			Hint:    w.Binary + " worktree list",
		}
	}

	//
	// The containers and the database can be rebuilt in seconds; uncommitted code cannot be
	// rebuilt at all, which is why git's own refusal is repeated here rather than worked around.
	//
	if !force && w.FS.IsDir(worktree.Path) && w.VCS.Dirty(worktree.Path) {
		return "", core.Refusal{
			Kind: "uncommitted_changes", Code: 6,
			Message: fmt.Sprintf("%s has uncommitted changes", name),
			Hint:    "Commit them, or repeat with --force",
		}
	}

	if interactive && !force {
		w.announce(fmt.Sprintf(
			"This destroys the environment %s and removes %s.\n\n", worktree.Project, worktree.Path))

		answer, err := w.Ask("Type the name to confirm", "")
		if err != nil || strings.TrimSpace(answer) != name {
			return "", nil
		}
	}

	w.announce(fmt.Sprintf("Removing %s...\n", name))

	//
	// The environment first, with its volumes: the database it was given was a copy, and leaving
	// it behind is how a machine fills up with the data of branches nobody works on any more.
	//
	if w.FS.IsDir(worktree.Path) {
		environment := core.Project{Name: worktree.Project, Root: worktree.Path}

		// With its volumes and its orphans: the environment is being erased, not stopped
		w.Orchestrator.Down(environment, files, //nolint:errcheck
			core.DownOptions{Volumes: true, RemoveOrphans: true})
	}

	w.VCS.RemoveWorktree(root, worktree.Path, force) //nolint:errcheck

	// A worktree whose directory a person deleted by hand leaves a stale administrative entry,
	// and git refuses to reuse the name until it is pruned
	w.VCS.Prune(root) //nolint:errcheck

	if err := w.Registry.Forget(parent, name); err != nil {
		return "", err
	}

	return name, nil
}

func (w Worktrees) announce(message string) {
	if w.Announce != nil {
		w.Announce(message)
	}
}

//
// Creating one.
//
// The proxy and the database templates are what make it affordable: a second environment needs no
// second set of ports, and its data is a file copy rather than an import. Without either, this is
// a command that promises an environment per branch and delivers a port clash and a forty-minute
// wait.
//

// The one proxy on the machine, and the network projects reach it through.
const (
	ProxyContainer = "hm-proxy"
	ProxyNetwork   = "hm-gateway"
)

// AddOptions is what a branch environment was asked for.
type AddOptions struct {
	Branch    string
	Profile   string
	Path      string
	Start     bool
	Anonymise bool
}

// Added is the environment that came out.
type Added struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Profile string `json:"profile"`
	Path    string `json:"path"`
	Project string `json:"project"`
	URL     string `json:"url"`
}

// Plan is everything decided before anything is created, which is what the caller then carries
// out — the parts that need a compose configuration, a database template and a running
// environment do not belong in here.
type Plan struct {
	Name    string
	Slug    string
	Project string
	Domain  string
	Path    string
	Profile string
	branch  string

	// SharedVendor says whether the dependencies of the main checkout can be read rather than
	// installed again, which is true only while they are the same dependencies.
	SharedVendor bool
	Mounts       []string
}

// Prepare checks everything that can be checked before anything is created.
//
// All of it before, and none of it after: half a branch environment — a worktree with no
// registration, a registration with no overlay — is the state nothing else in this tool knows how
// to talk about.
func (w Worktrees) Prepare(project core.Project, usesProxy bool, options AddOptions) (Plan, error) {
	if options.Branch == "" {
		return Plan{}, core.Refusal{
			Kind: "missing_branch", Code: 2,
			Message: "Which branch should get an environment?",
			Hint:    w.Binary + " worktree add feature/checkout",
		}
	}

	if _, ok := core.ProfileKeeps(options.Profile); !ok {
		return Plan{}, core.Refusal{
			Kind: "unknown_profile", Code: 2,
			Message: fmt.Sprintf("'%s' is not a profile", options.Profile),
			Hint:    "One of: " + core.Profiles(),
		}
	}

	if project.InWorktree {
		return Plan{}, core.Refusal{
			Kind: "not_the_main_checkout", Code: 6,
			Message: "Branch environments are created from the main checkout",
			Hint:    fmt.Sprintf("cd %s && %s worktree add <branch>", project.Root, w.Binary),
		}
	}

	//
	// Without the proxy every branch environment would publish its own ports, which is the
	// collision the proxy was built to end — with as many environments as branches this time.
	//
	if !usesProxy {
		return Plan{}, core.Refusal{
			Kind: "proxy_required", Code: 6,
			Message: "Branch environments are reached by name, which needs the global proxy",
			Hint:    fmt.Sprintf("%s proxy up && %s setup -f", w.Binary, w.Binary),
		}
	}

	slug := core.Slug(options.Branch)
	if slug == "" {
		return Plan{}, core.Refusal{
			Kind: "unusable_branch_name", Code: 2,
			Message: fmt.Sprintf("'%s' leaves nothing that can be used as a name", options.Branch),
			Hint:    fmt.Sprintf("%s worktree add %s --path=<dir>", w.Binary, options.Branch),
		}
	}

	path := options.Path
	if path == "" {
		path = filepath.Join(filepath.Dir(project.Root),
			filepath.Base(project.Root)+"-worktrees", slug)
	}

	name := core.Slug(filepath.Base(path))
	domain := name + "." + orLocalhost(project.Domain)
	child := project.Name + "-" + name

	if err := w.free(project, name, child, domain, path, options.Branch); err != nil {
		return Plan{}, err
	}

	return Plan{
		Name: name, Slug: slug, Project: child, Domain: domain,
		Path: path, Profile: options.Profile, branch: options.Branch,
	}, nil
}

// free refuses a name, a project, an address or a path that somebody else has.
//
// Names are refused, never resolved. Appending a number would give the environment a name nobody
// chose — and here the name decides which containers, which volumes and which database are used,
// so a name nobody chose is how somebody ends up working in an environment they did not mean to
// open.
func (w Worktrees) free(project core.Project, name, child, domain, path, branch string) error {
	existing, err := w.Registry.Worktree(project.Name, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Branch == branch {
			return core.Refusal{
				Kind: "already_registered", Code: 2,
				Message: fmt.Sprintf("'%s' already has an environment at %s", name, existing.Path),
				Hint:    w.Binary + " worktree list",
			}
		}

		// Two different branches whose names reduce to the same thing: feature/x and Feature_X
		return core.Refusal{
			Kind: "name_taken", Code: 2,
			Message: fmt.Sprintf("'%s' would be called '%s', and that name belongs to '%s'",
				branch, name, existing.Branch),
			Hint: fmt.Sprintf("%s worktree add %s --path=<another directory>", w.Binary, branch),
		}
	}

	containers, err := w.Engine.Containers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		if container.ComposeProject == child {
			return core.Refusal{
				Kind: "project_taken", Code: 2,
				Message: fmt.Sprintf("There is already an environment called '%s' on this machine", child),
				Hint:    w.Binary + " list",
			}
		}
	}

	if w.FS.Exists(path) {
		return core.Refusal{
			Kind: "path_in_use", Code: 2,
			Message: path + " already exists",
			Hint:    fmt.Sprintf("%s worktree add %s --path=<somewhere else>", w.Binary, branch),
		}
	}

	return nil
}

// Create makes the worktree, decides about the dependencies, and writes the registration and the
// overlay under one lock.
//
// Under one lock because without it the four steps — is the name free, create the worktree, write
// the overlay, write the registration — are four moments in which another agent can do the same
// thing, and both end up believing they own it.
func (w Worktrees) Create(project core.Project, plan Plan, services []string, workdir string) (Plan, error) {
	w.announce("Creating the worktree...\n")

	if err := w.FS.MkdirAll(filepath.Dir(plan.Path)); err != nil {
		return plan, err
	}

	output, err := w.VCS.AddWorktree(project.Root, plan.Path, plan.Branch())
	if err != nil || !w.FS.IsDir(plan.Path) {
		return plan, core.Refusal{
			Kind: "worktree_failed", Code: 6,
			Message: "git could not create the worktree: " + lastLine(output),
			Hint:    "git -C " + project.Root + " worktree list",
		}
	}

	//
	// Decided before the overlay is written, because the overlay is where the answer takes
	// effect: the dependencies of the main checkout are mounted where PHP expects to find them,
	// or the worktree installs its own.
	//
	if w.Platform != "mac" && w.sharesDependencies(project.Root, plan.Path) {
		plan.SharedVendor = true
		plan.Mounts = w.dependencyMounts(project.Root, workdir)
	}

	lock, err := w.Lock()
	if err != nil {
		return plan, core.Refusal{
			Kind: "locked", Code: 6,
			Message: err.Error(),
			Hint:    "Try again in a moment",
		}
	}
	defer lock()

	overlay := core.OverlayFor(w.Binary, plan.Project, plan.Profile, plan.Domain,
		ProxyNetwork, services, plan.Mounts)

	if err := w.Registry.WriteOverlay(project.Name, plan.Name, overlay); err != nil {
		return plan, err
	}

	return plan, w.Registry.Save(project.Name, core.Worktree{
		Name: plan.Name, Branch: plan.Branch(), Parent: project.Name,
		Profile: plan.Profile, Project: plan.Project, Domain: plan.Domain,
		Path: plan.Path, SharedVendor: plan.SharedVendor,
	})
}

// sharesDependencies answers whether the main checkout's can be read rather than installed again.
//
// Only while they are the same dependencies. Equal locks mean identical trees and sharing is free;
// different locks mean the branch changed them and sharing would be a lie.
func (w Worktrees) sharesDependencies(main, worktree string) bool {
	if !w.FS.IsDir(filepath.Join(main, "vendor")) {
		return false
	}

	theirs := w.FS.Read(filepath.Join(main, "composer.lock"))
	ours := w.FS.Read(filepath.Join(worktree, "composer.lock"))

	return theirs != "" && theirs == ours
}

// dependencyMounts are read-only, and they are mounts and never links.
//
// Composer's autoloader computes its base directory from `dirname($vendorDir)`, and PHP resolves
// `__DIR__` to the real path behind a symlink: with a link, the worktree's own modules are never
// registered and its code never runs. Read-only is what stops a `composer require` in one branch
// from corrupting what the main checkout and five other worktrees are reading.
//
// Only vendor and node_modules. generated, var and pub/static are compiled per branch, and a class
// from another branch is the hardest kind of bug to see.
func (w Worktrees) dependencyMounts(main, workdir string) []string {
	if workdir == "" {
		workdir = "/var/www/html"
	}

	mounts := []string{}

	for _, directory := range []string{"vendor", "node_modules"} {
		if !w.FS.IsDir(filepath.Join(main, directory)) {
			continue
		}

		mounts = append(mounts, fmt.Sprintf("%s/%s:%s/%s:ro", main, directory, workdir, directory))
	}

	return mounts
}

// Branch is what the plan was made for.
func (p Plan) Branch() string { return p.branch }

func lastLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	return lines[len(lines)-1]
}

func orLocalhost(domain string) string {
	if domain == "" {
		return "localhost"
	}

	return domain
}
