package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Pull is ai-pull: everything the configuration asks for, from every repository, again. What the
// tool installed is updated; what somebody made is left alone unless force says otherwise.
func (a *AITools) Pull(force bool) int {
	a.say(AIInfo, "Loading configuration...")

	config, raw, err := a.readJSON(core.AIPropertiesFile)
	if err != nil {
		if errors.Is(err, core.ErrAIInvalidJSON) {
			a.captured(AIError, "Invalid JSON in "+core.AIPropertiesFile)
		}

		a.Console.Say(AIError, "Failed to load configuration", AILine)

		return 1
	}

	if raw == "{}" {
		a.Console.Say(AIError, "No configuration found. Run 'hm ai-init' first.", AILine)

		return 1
	}

	object, _ := config.(*core.AIObject)
	platformsValue, _ := object.Get("platforms")
	resourcesValue, _ := object.Get("resources")
	typesValue, _ := object.Get("types")

	if object == nil || !core.AITruthy(platformsValue) || !core.AITruthy(resourcesValue) {
		a.Console.Say(AIError, "Invalid configuration in ai-properties.json", AILine)

		return 1
	}

	// `.x | join(",")` on something that is not a list stopped the script there, with jq's
	// complaint and jq's exit code
	for _, value := range []any{platformsValue, typesValue, resourcesValue} {
		if _, ok := value.([]any); !ok {
			a.Console.Stderr(fmt.Sprintf("jq: error: Cannot iterate over %s\n", jqType(value)))

			return 5
		}
	}

	platforms, types, resources := core.AIJoin(platformsValue), core.AIJoin(typesValue), core.AIJoin(resourcesValue)

	if platforms == "" || resources == "" {
		a.Console.Say(AIError, "Configuration incomplete: platforms and resources are required", AILine)

		return 1
	}

	shown := types
	if shown == "" {
		shown = "all"
	}

	a.say(AIInfo, "Configuration loaded:")
	a.say(AIInfo, "  Platforms: "+platforms)
	a.say(AIInfo, "  Types: "+shown)
	a.say(AIInfo, "  Resources: "+resources)
	a.Console.Stderr("\n")

	a.say(AIInfo, "Resolving repositories...")

	repositories, ok := a.repositories(a.Repositories, config)
	if !ok {
		a.Console.Say(AIError, "Failed to resolve repositories", AILine)

		return 1
	}

	a.say(AIInfo, "Starting download and installation...")

	if !a.pullAll(repositories, platforms, types, resources, force) {
		a.Console.Say(AIWarning, "Some operations failed, but continuing...", AILine)
	}

	a.Console.Stderr("\n")
	a.say(AIInfo, "AI tools update complete!")
	a.Console.Stderr("\n")
	a.say(AIInfo, "Registration: "+core.AIRegistrationFile)
	a.Console.Stderr("\n")

	if force {
		a.Console.Say(AIWarning, "Used --force flag: custom skills/agents may have been overwritten", AILine)
	} else {
		a.say(AIInfo, "Custom skills/agents were preserved")
		a.say(AIInfo, "Use --force to overwrite custom files")
	}

	return 0
}

func jqType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case *core.AIObject:
		return "object"
	default:
		return "number"
	}
}

func (a *AITools) pullAll(repositories []any, platforms, types, resources string, force bool) bool {
	catalog := a.catalog()

	if len(repositories) == 0 {
		a.say(AIInfo, "No repositories configured")

		return true
	}

	temp := a.tempBase()
	defer os.RemoveAll(temp) //nolint:errcheck

	a.say(AIInfo, fmt.Sprintf("Downloading from %d repositories...", len(repositories)))

	succeeded, failed := 0, 0

	for _, value := range repositories {
		repository := core.ReadAIRepository(value, "")
		filter := types

		a.say(AIInfo, "Processing repository: "+repository.Name)

		a.origin, a.url, a.branch, a.version = repository.Name, repository.URL, repository.Branch, ""

		var dir string

		// The skills that come with the tool are installed from the installed copy of it, not
		// downloaded, and not filtered by type: they describe the tool in use
		if repository.Local {
			dir, filter = a.Tool, ""
			a.branch = ""

			if a.Version != nil {
				a.version = a.Version()
			}
		} else {
			dir = temp + "/" + repository.Name
			if !a.download(repository.URL, repository.Branch, dir) {
				a.Console.Say(AIWarning, "Failed to download "+repository.Name+", skipping...", AILine)

				failed++

				continue
			}
		}

		if !a.validStructure(dir) {
			a.Console.Say(AIWarning, "Invalid repository structure in "+repository.Name+", skipping...", AILine)

			failed++

			continue
		}

		installedAny := false

		for _, platform := range split(platforms) {
			for _, kind := range split(resources) {
				directory := catalog.Directory(platform, kind)
				if directory == "" {
					continue
				}

				a.say(AIInfo, "Installing "+kind+" for "+platform+"...")

				if a.install(dir, kind, directory, filter, force) {
					installedAny = true
				}
			}
		}

		if installedAny {
			succeeded++
		} else {
			failed++
		}
	}

	a.say(AIInfo, fmt.Sprintf("Processed %d repositories (%d successful, %d failed)", len(repositories), succeeded, failed))

	if failed > 0 {
		a.Console.Say(AIWarning, "Some repositories failed to download or install", AILine)

		return false
	}

	return true
}
