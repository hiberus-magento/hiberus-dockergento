// Package composelib runs Compose as a library.
//
// Not as a subprocess: `github.com/docker/compose/v2` exposes the same engine the `docker compose`
// command drives, and calling it here means the tool no longer starts a process, re-reads the
// files Compose already read once, and reads back a terminal to find out what happened.
//
// The property that makes this safe was measured rather than assumed: an environment created
// through this package and one created by `docker compose up` carry the same configuration hash,
// so neither recreates the other's containers and `docker compose ps` sees both. That is the risk
// this decision turned on, and there is a test that keeps it true.
//
// What it buys beyond the time is the shape of the answers. Progress and logs arrive as calls
// rather than as lines on a terminal, which is what an HTTP or MCP adapter would need — and what
// no amount of parsing the command's output would have given.
package composelib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	command "github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Orchestrator is Compose.
type Orchestrator struct {
	// Environment is what variables in the files interpolate against, which is the project's
	// properties on top of the process's — what `set -a` did in the shell implementation.
	Environment map[string]string

	// Where the operation reports to. Compose renders its own progress and its own log prefixes
	// into these, which is why the output is the output people already know.
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

func (o Orchestrator) Up(project core.Project, files core.ComposeFiles, services []string) error {
	service, loaded, err := o.open(project, files)
	if err != nil {
		return err
	}

	return service.Up(context.Background(), loaded, api.UpOptions{
		Create: api.CreateOptions{
			// Asking for the build step is what resolves each service's image and records its
			// digest, and the digest is half of the decision to replace a container. Without it
			// a pulled image would never reach a running environment: `docker pull` and then
			// `start` would report everything up to date and keep the old one
			Build:    &api.BuildOptions{Deps: true, Services: services},
			Services: services,
			// Compose's own defaults for `up`: reuse anonymous volumes, and only recreate what
			// has actually changed. Anything else here would silently destroy data on a command
			// people run several times a day
			Recreate:             api.RecreateDiverged,
			RecreateDependencies: api.RecreateDiverged,
			Inherit:              true,
		},
		Start: api.StartOptions{
			Project:  loaded,
			Services: services,
		},
	})
}

func (o Orchestrator) Stop(project core.Project, files core.ComposeFiles, services []string) error {
	service, loaded, err := o.open(project, files)
	if err != nil {
		return err
	}

	return service.Stop(context.Background(), loaded.Name, api.StopOptions{
		Project:  loaded,
		Services: services,
	})
}

// Down removes an environment: its containers, its networks and, when asked, its data.
//
// With the volumes is what a branch environment wants when it goes: the database it was given was
// a copy, and leaving it behind is how a machine fills up with the data of branches nobody works
// on any more.
func (o Orchestrator) Down(project core.Project, files core.ComposeFiles, volumes bool) error {
	service, loaded, err := o.open(project, files)
	if err != nil {
		return err
	}

	return service.Down(context.Background(), loaded.Name, api.DownOptions{
		Project:       loaded,
		Volumes:       volumes,
		RemoveOrphans: true,
	})
}

// Logs writes the logs of the services named, with Compose's own consumer.
//
// Its own and not one of ours: the prefix width, the colour each service is given and the way a
// container that appears mid-stream is picked up are all decisions somebody already made, and
// making them again differently would only mean output that looks almost the same.
func (o Orchestrator) Logs(project core.Project, files core.ComposeFiles, services []string, options core.LogOptions) error {
	service, loaded, err := o.open(project, files)
	if err != nil {
		return err
	}

	ctx := context.Background()
	consumer := formatter.NewLogConsumer(ctx, o.out(), o.err(), !options.NoColor, !options.NoPrefix, options.Timestamps)

	return service.Logs(ctx, loaded.Name, consumer, api.LogOptions{
		Project:    loaded,
		Services:   services,
		Follow:     options.Follow,
		Index:      options.Index,
		Tail:       options.Tail,
		Since:      options.Since,
		Until:      options.Until,
		Timestamps: options.Timestamps,
	})
}

// Exec runs a command inside a running service and returns its exit code.
func (o Orchestrator) Exec(project core.Project, files core.ComposeFiles, name string, arguments []string, options core.ExecOptions) (int, error) {
	service, loaded, err := o.open(project, files)
	if err != nil {
		return 0, err
	}

	return service.Exec(context.Background(), loaded.Name, api.RunOptions{
		Project:     loaded,
		Service:     name,
		Command:     arguments,
		User:        options.User,
		Detach:      options.Detach,
		Tty:         options.Tty,
		Interactive: options.Interactive,
		Privileged:  options.Privileged,
		WorkingDir:  options.Workdir,
		Index:       options.Index,
	})
}

// open builds the Compose service and loads the project it will act on.
func (o Orchestrator) open(project core.Project, files core.ComposeFiles) (api.Compose, *types.Project, error) {
	docker, err := command.NewDockerCli(
		command.WithInputStream(o.in()),
		command.WithOutputStream(o.out()),
		command.WithErrorStream(o.err()),
	)
	if err != nil {
		return nil, nil, err
	}

	// The docker CLI's own initialisation, which is also what resolves the daemon: the context
	// store, DOCKER_HOST, the config file. Doing it any other way is how a binary reports "Docker
	// is not running" on every machine using Colima
	if err := docker.Initialize(flags.NewClientOptions()); err != nil {
		return nil, nil, err
	}

	//
	// Whether anything is coloured, decided the way the `docker compose` command decides it and
	// by the same function: automatic means "is the output a terminal", and `COMPOSE_ANSI`
	// overrides it. Left out, everything came out coloured even when the output was a pipe — and
	// the escape sequences are invisible until something compares the bytes.
	//
	ansi := formatter.Auto
	if mode, set := os.LookupEnv("COMPOSE_ANSI"); set {
		ansi = mode
	}

	formatter.SetANSIMode(docker, ansi)

	loaded, err := o.load(project, files)
	if err != nil {
		return nil, nil, err
	}

	return compose.NewComposeService(docker), loaded, nil
}

func (o Orchestrator) load(project core.Project, files core.ComposeFiles) (*types.Project, error) {
	paths := make([]string, 0, len(files.Load))

	for _, file := range files.Load {
		if file == "" {
			continue
		}

		if !filepath.IsAbs(file) {
			file = filepath.Join(project.Root, file)
		}

		// The machine overlay of the other platform is not there on this one, and the shell
		// implementation skips it the same way
		if _, err := os.Stat(file); err != nil {
			continue
		}

		paths = append(paths, file)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose file to read in %s", project.Root)
	}

	options, err := cli.NewProjectOptions(paths,
		cli.WithWorkingDirectory(project.Root),
		cli.WithName(project.Name),
		cli.WithEnv(flatten(o.Environment)),
		cli.WithResolvedPaths(true),
	)
	if err != nil {
		return nil, err
	}

	loaded, err := options.LoadProject(context.Background())
	if err != nil {
		return nil, err
	}

	//
	// The labels Compose stamps on everything it creates, set exactly as its own CLI sets them.
	//
	// This is the part that must not drift. They are what `docker compose ps` matches on, what
	// `down` uses to know what to remove, and what tells a container created by this tool from
	// one created by hand. Written out here rather than reimplemented elsewhere so that there is
	// one place to compare against upstream.
	//
	for name, service := range loaded.Services {
		service.CustomLabels = map[string]string{
			api.ProjectLabel:     loaded.Name,
			api.ServiceLabel:     name,
			api.VersionLabel:     api.ComposeVersion,
			api.WorkingDirLabel:  loaded.WorkingDir,
			api.ConfigFilesLabel: strings.Join(loaded.ComposeFiles, ","),
			api.OneoffLabel:      "False",
		}

		loaded.Services[name] = service
	}

	return loaded, nil
}

func (o Orchestrator) in() *os.File {
	if o.Stdin != nil {
		return o.Stdin
	}

	return os.Stdin
}

func (o Orchestrator) out() *os.File {
	if o.Stdout != nil {
		return o.Stdout
	}

	return os.Stdout
}

func (o Orchestrator) err() *os.File {
	if o.Stderr != nil {
		return o.Stderr
	}

	return os.Stderr
}

func flatten(environment map[string]string) []string {
	entries := make([]string, 0, len(environment))
	for key, value := range environment {
		entries = append(entries, key+"="+value)
	}

	return entries
}

// The version Compose stamps on everything it creates, which in a library build is empty: it is
// filled in by the compose CLI at its own build time and there is no CLI here.
//
// It is read from this binary's own dependency list rather than written down, so that bumping the
// library cannot leave the label claiming a version nothing here is running.
func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, dependency := range info.Deps {
		if dependency.Path == "github.com/docker/compose/v2" {
			api.ComposeVersion = strings.TrimPrefix(dependency.Version, "v")

			return
		}
	}
}
