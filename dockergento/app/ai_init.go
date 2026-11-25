package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Init is ai-init: the configuration, from the options or from the wizard when there are none,
// the platforms' directories, and a first installation.
func (a *AITools) Init(options AIInitOptions, interactive bool) int {
	if options.Plugin != "" {
		return a.Plugin(options.Plugin)
	}

	asked := options.Platforms == "" && options.Types == "" && options.Resources == "" && options.Repository == ""

	existing, _, err := a.readJSON(core.AIPropertiesFile)
	if err != nil {
		if errors.Is(err, core.ErrAIInvalidJSON) {
			a.captured(AIError, "Invalid JSON in "+core.AIPropertiesFile)
		}

		a.Console.Say(AIError, "Failed to load existing configuration", AIDecorated)

		return 1
	}

	var config *core.AIObject
	if asked {
		config = a.wizard(existing, interactive)
	} else {
		var custom any = []any{}
		if options.Repository != "" {
			custom = []any{core.AICustomRepository(core.AIRepositoryName(options.Repository), options.Repository, options.Branch)}
		}

		resources := options.Resources
		if resources == "" {
			resources = "skills,agents"
		}

		// The usage offers "both": skills and agents, not a resource of that name, which no
		// platform has a directory for (the script saved it as given and installed nothing)
		var expanded []string

		seen := map[string]bool{}

		for _, resource := range strings.Split(resources, ",") {
			names := []string{resource}
			if resource == "both" {
				names = []string{"skills", "agents"}
			}

			for _, name := range names {
				if !seen[name] {
					seen[name] = true
					expanded = append(expanded, name)
				}
			}
		}

		resources = strings.Join(expanded, ",")

		config = core.AIConfiguration(resources, options.Platforms, options.Types, custom, a.now())
	}

	platformsValue, _ := config.Get("platforms")
	typesValue, _ := config.Get("types")
	resourcesValue, _ := config.Get("resources")
	platforms, types, resources := core.AIJoin(platformsValue), core.AIJoin(typesValue), core.AIJoin(resourcesValue)

	if platforms == "" || resources == "" {
		a.Console.Say(AIError, "Configuration incomplete: platforms and resources are required", AIDecorated)

		return 1
	}

	a.Console.Say(AIInfo, "Saving configuration...\n", AIDecorated)

	if a.writeJSON(core.AIPropertiesFile, config) != nil {
		a.Console.Say(AIError, "Failed to save configuration", AIDecorated)

		return 1
	}

	a.Console.Say(AIInfo, "Configuration saved to "+core.AIPropertiesFile+"\n", AIDecorated)

	a.Console.Say(AIInfo, "Creating platform directories...\n", AIDecorated)
	a.createDirectories(platforms, resources)

	a.Console.Say(AIInfo, "Resolving repositories...\n", AIDecorated)

	repositories, ok := a.repositories("", config)
	if !ok {
		a.Console.Say(AIError, "Failed to resolve repositories", AIDecorated)

		return 1
	}

	a.Console.Say(AIInfo, "Starting download and installation...\n", AIDecorated)
	a.initAll(repositories, platforms, types, resources)
	a.reportLegacy(AIDecorated)

	a.Console.Stdout("\n")
	a.Console.Say(AIInfo, "AI tools initialization complete!\n", AIDecorated)
	a.Console.Stdout("\n")
	a.Console.Say(AIInfo, "Configuration: "+core.AIPropertiesFile+"\n", AIDecorated)
	a.Console.Say(AIInfo, "Registration: "+core.AIRegistrationFile+"\n", AIDecorated)
	a.Console.Stdout("\n")
	a.Console.Say(AIInfo, "You can now use AI tools with your configured platforms.\n", AIDecorated)
	a.Console.Say(AIInfo, "Run 'hm ai-pull' to update tools in the future.\n", AIDecorated)

	return 0
}

func (a *AITools) createDirectories(platforms, resources string) {
	file := a.data("ai-platforms.json")
	if !a.hasData("ai-platforms.json") {
		a.Console.Say(AIError, "Platform definitions not found: "+file, AIDecorated)
		a.Console.Say(AIWarning, "Some directories could not be created", AIDecorated)

		return
	}

	catalog := a.catalog()

	for _, platform := range split(platforms) {
		for _, kind := range split(resources) {
			if directory := catalog.Directory(platform, kind); directory != "" {
				if os.MkdirAll(a.path(directory), 0o777) != nil {
					a.Console.Say(AIWarning, "Failed to create directory: "+directory, AIDecorated)
				}
			}
		}
	}
}

// initAll is ai-init's download_and_install, which predates the tool's own skills: it downloads
// every repository, the local one included — which has no URL, so it is refused and skipped.
func (a *AITools) initAll(repositories []any, platforms, types, resources string) {
	catalog := a.catalog()

	if len(repositories) == 0 {
		a.Console.Say(AIInfo, "No repositories configured", AIDecorated)

		return
	}

	temp := a.tempBase()
	defer os.RemoveAll(temp) //nolint:errcheck

	a.origin, a.url, a.branch, a.version = "", "", "", ""

	a.Console.Say(AIInfo, fmt.Sprintf("Downloading from %d repositories...\n", len(repositories)), AIDecorated)

	for _, value := range repositories {
		repository := core.ReadAIRepository(value, "null")

		a.Console.Say(AIInfo, "Processing repository: "+repository.Name+"\n", AIDecorated)

		// Deliberately unlike the script, which joined whatever name it was given
		if !core.ValidAIRepositoryName(repository.Name) {
			a.Console.Say(AIWarning, "Invalid repository name: "+repository.Name+", skipping...", AIDecorated)

			continue
		}

		dir := temp + "/" + repository.Name
		if !a.download(repository.URL, repository.Branch, dir) {
			a.Console.Say(AIWarning, "Failed to download "+repository.Name+", skipping...", AIDecorated)

			continue
		}

		if !a.validStructure(dir) {
			a.Console.Say(AIWarning, "Invalid repository structure in "+repository.Name+", skipping...", AIDecorated)

			continue
		}

		for _, platform := range split(platforms) {
			for _, kind := range split(resources) {
				directory := catalog.Directory(platform, kind)
				if directory == "" {
					continue
				}

				a.Console.Say(AIInfo, "Installing "+kind+" for "+platform+"...\n", AIDecorated)

				if !a.install(dir, kind, directory, types, false) {
					a.Console.Say(AIWarning, "Failed to install "+kind+" for "+platform+"\n", AIDecorated)
				}
			}
		}
	}

	a.Console.Say(AIInfo, "Download and installation complete\n", AIDecorated)
}

// ---------------------------------------------------------------------------------- the wizard

type aiOption struct{ id, name, description string }

// wizard is run_wizard: four questions, each starting from what the project already has.
func (a *AITools) wizard(existing any, interactive bool) *core.AIObject {
	a.Console.Stderr("\n")
	a.Console.Say(AIHeader, "========================================\nAI Tools Configuration Wizard\n========================================\n", AIDecoratedErr)
	a.Console.Stderr("\n")
	a.Console.Say(AIInfo, "Instructions: Enter numbers (1,2), IDs (claude,cursor), 'all', or press Enter to keep current", AIDecoratedErr)
	a.Console.Stderr("\n")

	object, _ := existing.(*core.AIObject)
	field := func(key string) any {
		value, _ := object.Get(key)

		return value
	}

	custom := field("custom_repositories")
	if custom == nil || custom == false {
		custom = []any{}
	}

	resources := a.menu("Which resources would you like to manage?", []aiOption{
		{"skills", "Skills", "AI assistant skills for common development tasks"},
		{"agents", "Agents", "Autonomous agents for complex workflows"},
	}, core.AIJoin(field("resources")), interactive)

	platforms := ""
	if options, ok := a.options("ai-platforms.json", "platforms", "Platform definitions not found: "); ok {
		platforms = a.menu("Which AI platforms do you use?", options, core.AIJoin(field("platforms")), interactive)
	}

	types := ""
	if options, ok := a.options("ai-skill-types.json", "skill_types", "Skill type definitions not found: "); ok {
		types = a.menu("Which skill types does your project need?", options, core.AIJoin(field("types")), interactive)
	}

	custom = a.customRepositories(custom, interactive)

	return core.AIConfiguration(resources, platforms, types, custom, a.now())
}

// options is the choices a data file offers, in the order it lists them.
func (a *AITools) options(file, key, missing string) ([]aiOption, bool) {
	path := a.data(file)

	contents, err := a.readData(file)
	if err != nil {
		a.Console.Say(AIError, missing+path, AIDecoratedErr)
		a.Console.Stderr("\n")

		return nil, false
	}

	value, _ := core.ParseAIJSON(contents)
	object, _ := value.(*core.AIObject)
	listed, _ := object.Get(key)
	entries, _ := listed.(*core.AIObject)

	var options []aiOption

	for _, id := range entries.Keys() {
		entry, _ := entries.Get(id)
		fields, _ := entry.(*core.AIObject)
		name, _ := fields.Get("name")
		description, _ := fields.Get("description")

		options = append(options, aiOption{id, core.AIText(name, "null"), core.AIText(description, "null")})
	}

	return options, true
}

// menu is multi_select_menu: numbers, ids, "all", or nothing to keep what was there.
func (a *AITools) menu(prompt string, options []aiOption, preselected string, interactive bool) string {
	a.Console.Stderr("\n")
	a.Console.Say(AIInfo, prompt, AIDecoratedErr)
	a.Console.Stderr("\n")

	for i, option := range options {
		box := "[ ]"
		if strings.Contains(","+preselected+",", ","+option.id+",") {
			box = "[X]"
		}

		a.Console.Stderr(fmt.Sprintf("  %s %d) %s - %s\n", box, i+1, option.name, option.description))
	}

	a.Console.Stderr("\n")
	a.Console.Stderr("> ")

	if !interactive {
		a.Console.Fail(2, "input_required", "Non-interactive mode cannot run the AI tools wizard",
			a.Binary+" ai-init --platforms=... --types=... --resources=...")

		return ""
	}

	answer := a.Console.ReadLine()

	ids := make([]string, 0, len(options))
	for _, option := range options {
		ids = append(ids, option.id)
	}

	switch answer {
	case "all":
		return strings.Join(ids, ",")
	case "":
		return preselected
	}

	var chosen []string

	for _, selection := range strings.Split(answer, ",") {
		selection = strings.Join(strings.Fields(selection), " ")
		if selection == "" {
			continue
		}

		if number, ok := digits(selection); ok {
			if number >= 1 && number <= len(ids) {
				chosen = append(chosen, ids[number-1])
			}

			continue
		}

		for _, id := range ids {
			if id == selection {
				chosen = append(chosen, id)

				break
			}
		}
	}

	return strings.Join(chosen, ",")
}

func digits(text string) (int, bool) {
	number := 0

	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, false
		}

		if number < 1<<30 {
			number = number*10 + int(r-'0')
		}
	}

	return number, true
}

func yes(answer string) bool { return strings.HasPrefix(answer, "y") || strings.HasPrefix(answer, "Y") }

// customRepositories is wizard_custom_repositories: one more repository, if anybody wants one.
func (a *AITools) customRepositories(existing any, interactive bool) any {
	a.Console.Stderr("\n")
	a.Console.Say(AIInfo, "Would you like to add a custom repository? (y/N)", AIDecoratedErr)
	a.Console.Stderr("\n")

	if !interactive || !yes(a.Console.ReadLine()) {
		return existing
	}

	ask := func(question string) string {
		a.Console.Stderr("\n")
		a.Console.Say(AIInfo, question, AIDecoratedErr)
		a.Console.Stderr("\n")

		return a.Console.ReadLine()
	}

	url := ask("Enter repository URL (must be HTTPS):")
	if !validURL(url) {
		a.Console.Say(AIError, "Repository URL must use HTTPS protocol", AIDecoratedErr)
		a.Console.Stderr("\n")

		return existing
	}

	branch := ask("Enter branch name (default: main):")
	if branch == "" {
		branch = "main"
	}

	name := ask("Enter repository name (optional, for display):")
	if name == "" {
		name = core.AIRepositoryName(url)
	}

	list, _ := existing.([]any)

	return append(append([]any{}, list...), core.AICustomRepository(name, url, branch))
}
