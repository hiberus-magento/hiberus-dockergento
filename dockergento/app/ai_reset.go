package app

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Reset is ai-reset: everything the registration says the tool installed, and nothing else.
func (a *AITools) Reset(confirm, interactive bool) int {
	a.say(AIInfo, "Loading registration data...")

	registration, ok := a.registration()
	if !ok {
		a.Console.Say(AIError, "Failed to load ai-registration.json", AILine)
		a.Console.Say(AIError, "Registration file may be corrupted or missing", AILine)
		a.say(AIInfo, "If the file is corrupted, manually delete "+core.AIRegistrationFile)

		return 1
	}

	skills, _ := registration.Get("skills")
	agents, _ := registration.Get("agents")

	if length(skills) == 0 && length(agents) == 0 {
		a.say(AIInfo, "No tracked AI tools found in registration")
		a.say(AIInfo, "Nothing to reset")

		return 0
	}

	a.say(AIInfo, "Collecting tracked files...")

	var files []string

	for _, kind := range []any{skills, agents} {
		entries, _ := kind.(*core.AIObject)

		keys := entries.Keys()
		sort.Strings(keys)

		for _, path := range keys {
			if path != "" {
				files = append(files, path)
			}
		}
	}

	count := 0

	for _, file := range files {
		for _, line := range strings.Split(file, "\n") {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
	}

	if count == 0 {
		a.say(AIInfo, "No files to remove")

		return 0
	}

	a.Console.Stdout("\n")
	a.Console.Say(AIWarning, fmt.Sprintf("The following %d items will be removed:", count), AILine)
	a.Console.Stdout(strings.Join(files, "\n") + "\n")
	a.Console.Stdout("\n")

	a.say(AIInfo, "Custom skills/agents will be preserved:")

	if aborted := a.preserved(); aborted != 0 {
		return aborted
	}

	if !confirm && interactive {
		a.Console.Stdout("\n")
		a.Console.Say(AIWarning, "This action cannot be undone.", AILine)
		a.Console.Stdout("Continue with deletion? (y/N) ")

		if !yes(a.Console.ReadLine()) {
			a.say(AIInfo, "Reset cancelled")

			return 0
		}
	}

	a.Console.Stdout("\n")
	a.say(AIInfo, "Removing tracked files...")

	removed := 0

	for _, file := range files {
		path := a.path(file)

		info, err := os.Stat(path)
		if err != nil {
			a.Console.Say(AIWarning, "  File not found (already removed?): "+file, AILine)

			continue
		}

		switch {
		case info.IsDir():
			if os.RemoveAll(path) == nil {
				a.say(AIInfo, "  Removed directory: "+file)
				removed++
			} else {
				a.Console.Say(AIError, "  Failed to remove directory: "+file, AILine)
			}
		case info.Mode().IsRegular():
			if os.Remove(path) == nil {
				a.say(AIInfo, "  Removed file: "+file)
				removed++
			} else {
				a.Console.Say(AIError, "  Failed to remove file: "+file, AILine)
			}
		}
	}

	a.say(AIInfo, "Clearing registration...")

	if !a.saveRegistration(emptyRegistration()) {
		a.Console.Say(AIError, "Failed to clear registration file", AILine)

		return 1
	}

	a.Console.Stdout("\n")
	a.say(AIInfo, "AI tools reset complete!")
	a.say(AIInfo, fmt.Sprintf("Removed %d items", removed))
	a.Console.Stdout("\n")
	a.say(AIInfo, "Configuration file preserved: "+core.AIPropertiesFile)
	a.say(AIInfo, "Registration file cleared: "+core.AIRegistrationFile)
	a.Console.Stdout("\n")
	a.say(AIInfo, "To re-download AI tools, run: hm ai-pull")

	return 0
}

// length is jq's length, for what a registration can hold.
func length(value any) int {
	switch typed := value.(type) {
	case *core.AIObject:
		return typed.Len()
	case []any:
		return len(typed)
	case string:
		return len([]rune(typed))
	case nil, bool:
		return 0
	default:
		return 1
	}
}

// preserved is show_preserved_summary: how many of each platform's skills and agents are somebody's
// own, and will still be there afterwards. Not zero is the exit code the script stopped with there.
func (a *AITools) preserved() int {
	file := a.data("ai-platforms.json")
	if info, err := os.Stat(file); err != nil || info.IsDir() {
		return 0
	}

	catalog := a.catalog()

	config, _, err := a.readJSON(core.AIPropertiesFile)
	if err != nil {
		// `config=$(load_ai_properties) || echo "{}"`: the fallback went to stdout, not into the
		// variable
		a.Console.Stdout("{}\n")

		if !errors.Is(err, core.ErrAIInvalidJSON) {
			return 0
		}

		// Under --json the complaint went to stderr and jq read nothing. In text it went into
		// the variable, jq was handed that sentence, and set -e stopped the script on jq's
		// failure — before anything was removed
		if a.Console.JSON() {
			a.Console.Say(AIError, "Invalid JSON in "+core.AIPropertiesFile, AIDecorated)

			return 0
		}

		a.Console.Stderr("jq: parse error: Invalid numeric literal at line 1, column 8\n")

		return 5
	}

	object, _ := config.(*core.AIObject)
	platformsValue, _ := object.Get("platforms")

	total := 0

	for _, platform := range split(core.AIJoin(platformsValue)) {
		for _, kind := range []string{"skills", "agents"} {
			directory := catalog.Directory(platform, kind)
			if directory == "" {
				continue
			}

			custom := a.custom(directory, kind)
			total += custom

			if custom > 0 {
				a.say(AIInfo, fmt.Sprintf("  %s custom %s: %d", platform, kind, custom))
			}
		}
	}

	if total > 0 {
		a.Console.Stdout("\n")
		a.say(AIInfo, fmt.Sprintf("Total custom items that will be preserved: %d", total))
	}

	return 0
}

// custom is count_custom_items: the directories in a platform's directory the registration does
// not name.
func (a *AITools) custom(directory, kind string) int {
	if !isDirectory(a.path(directory)) {
		return 0
	}

	count := 0

	for _, name := range visible(a.path(directory)) {
		if isDirectory(a.path(directory+"/"+name)) && !a.tracked(kind, directory+"/"+name) {
			count++
		}
	}

	return count
}
