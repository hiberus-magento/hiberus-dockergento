// Package composecfg reads a project's Compose configuration.
//
// With the library Compose itself uses to parse, merge and interpolate, rather than by running
// `docker compose config` and parsing its output. That call costs between 90 and 260 ms and the
// shell implementation makes it fourteen times across the tool; here it is a typed read with no
// subprocess at all.
//
// What it does not do is run anything. Bringing an environment up stays with the `docker compose`
// command: embedding the engine is 395 modules and an 84 MB binary, and reimplementing it means
// reimplementing the labels and the configuration hash that decide whether a container belongs to
// a project — get those wrong and `docker compose ps` stops seeing what we created.
package composecfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Loader reads the configuration of a project.
type Loader struct {
	// Environment is what variables in the files interpolate against. Empty means the process's.
	Environment map[string]string
}

// Load resolves the given compose files, in order, as one project.
func (l Loader) Load(root, name string, files []string) (core.Compose, error) {
	paths := make([]string, 0, len(files))

	for _, file := range files {
		if file == "" {
			continue
		}

		if !filepath.IsAbs(file) {
			file = filepath.Join(root, file)
		}

		// A file that is not there is not an error: the machine overlay of the other platform is
		// missing on this one, and the shell implementation skips it the same way
		if _, err := os.Stat(file); err != nil {
			continue
		}

		paths = append(paths, file)
	}

	if len(paths) == 0 {
		return core.Compose{}, fmt.Errorf("no compose file to read in %s", root)
	}

	environment := l.Environment
	if environment == nil {
		environment = map[string]string{}
		for _, entry := range os.Environ() {
			for at := 0; at < len(entry); at++ {
				if entry[at] == '=' {
					environment[entry[:at]] = entry[at+1:]

					break
				}
			}
		}
	}

	options, err := cli.NewProjectOptions(paths,
		cli.WithWorkingDirectory(root),
		cli.WithName(name),
		cli.WithEnv(flatten(environment)),
		cli.WithResolvedPaths(true),
	)
	if err != nil {
		return core.Compose{}, err
	}

	project, err := options.LoadProject(context.Background())
	if err != nil {
		return core.Compose{}, err
	}

	return convert(project), nil
}

func flatten(environment map[string]string) []string {
	entries := make([]string, 0, len(environment))
	for key, value := range environment {
		entries = append(entries, key+"="+value)
	}

	return entries
}

func convert(project *types.Project) core.Compose {
	compose := core.Compose{
		Name:     project.Name,
		Services: make([]core.Service, 0, len(project.Services)),
		Volumes:  map[string]string{},
	}

	for name, service := range project.Services {
		converted := core.Service{
			Name:        name,
			Image:       service.Image,
			Environment: map[string]string{},
		}

		for _, port := range service.Ports {
			converted.Ports = append(converted.Ports, core.Port{
				Published: port.Published,
				Target:    fmt.Sprintf("%d", port.Target),
			})
		}

		for key, value := range service.Environment {
			if value != nil {
				converted.Environment[key] = *value
			}
		}

		compose.Services = append(compose.Services, converted)
	}

	// Sorted by name, because a map has no order and the answer must not depend on which one the
	// runtime happened to walk first
	sort.Slice(compose.Services, func(a, b int) bool {
		return compose.Services[a].Name < compose.Services[b].Name
	})

	for name, volume := range project.Volumes {
		compose.Volumes[name] = volume.Name
	}

	return compose
}
