package cli

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/fsprops"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/internal/app"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// resolveProject answers what project the current directory belongs to.
func resolveProject() (core.Project, error) {
	directory, err := os.Getwd()
	if err != nil {
		return core.Project{}, err
	}

	resolver := app.Resolver{
		Properties: fsprops.Reader{Defaults: defaultsFile()},
		VCS:        gitvcs.Git{},
		Registry:   registry{},
	}

	return resolver.Resolve(directory)
}

// properties re-reads the project's file. It is a small JSON and the alternative is carrying a map
// through every layer that might one day need one value from it.
func properties(project core.Project) map[string]string {
	values, err := (fsprops.Reader{Defaults: defaultsFile()}).Load(project.Root)
	if err != nil {
		return map[string]string{}
	}

	return values
}

func property(project core.Project, key string) string {
	return properties(project)[key]
}

// composeFilesFor is the pair the shell implementation uses: the project's file and the overlay
// for this platform. The proxy and worktree overlays join them once those commands are ported.
func composeFilesFor(project core.Project) []string {
	values := properties(project)

	base := values["DOCKER_COMPOSE_FILE"]
	if base == "" {
		base = "docker-compose.yml"
	}

	overlay := values["DOCKER_COMPOSE_FILE_LINUX"]
	if runtime.GOOS == "darwin" {
		overlay = values["DOCKER_COMPOSE_FILE_MAC"]
	}

	if overlay == "" {
		overlay = "docker-compose.dev." + map[bool]string{true: "mac", false: "linux"}[runtime.GOOS == "darwin"] + ".yml"
	}

	return []string{base, overlay}
}

// environmentFor is what the variables inside the compose files interpolate against: the process's
// environment with the project's properties on top, which is what `set -a` did in the shell
// implementation.
func environmentFor(project core.Project) map[string]string {
	environment := map[string]string{}

	for _, entry := range os.Environ() {
		for at := 0; at < len(entry); at++ {
			if entry[at] == '=' {
				environment[entry[:at]] = entry[at+1:]

				break
			}
		}
	}

	for key, value := range properties(project) {
		environment[key] = value
	}

	environment["COMPOSE_PROJECT_NAME"] = project.Name
	environment["HM_ROOT"] = project.Root

	return environment
}

// defaultsFile is the tool's own properties, which a project's are merged over — the same merge
// the shell implementation does, and the reason a project that never set WORKDIR_PHP still has
// one.
func defaultsFile() string {
	root := hmRoot()
	if root == "" {
		return ""
	}

	return filepath.Join(root, "data", "properties.json")
}

// hmRoot is the tool's own directory, for the version it reports.
func hmRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	return filepath.Dir(filepath.Dir(executable))
}
