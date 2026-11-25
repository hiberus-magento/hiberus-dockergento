package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	// A registration is a file anybody can edit: skills and agents that are not objects are not
	// something the tool wrote, and nothing is removed on their word
	for _, kind := range []any{skills, agents} {
		if _, isObject := kind.(*core.AIObject); kind != nil && !isObject {
			a.Console.Say(AIError, "Invalid registration JSON structure (skills and agents must be objects)", AILine)

			return 1
		}
	}

	a.say(AIInfo, "Collecting tracked files...")

	var files, refused []string

	// kept is what the registration still holds afterwards: what was refused, and what could not
	// be removed
	kept := emptyRegistration()
	kindOf := map[string]string{}

	for _, kind := range []string{"skills", "agents"} {
		value, _ := registration.Get(kind)
		entries, _ := value.(*core.AIObject)

		keys := entries.Keys()
		sort.Strings(keys)

		for _, path := range keys {
			if path == "" {
				continue
			}

			if !a.removable(kind, path) {
				refused = append(refused, path)
				entry, _ := entries.Get(path)
				keptEntries, _ := kept.Get(kind)
				keptEntries.(*core.AIObject).Set(path, entry)

				continue
			}

			files = append(files, path)
			kindOf[path] = kind
		}
	}

	// Deliberately unlike the script, which removed whatever path the registration named
	for _, path := range refused {
		a.Console.Say(AIError, "  Refusing to remove "+path+": not inside the project's AI tools directories", AILine)
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

		if len(refused) > 0 {
			return 1
		}

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

		info, err := os.Lstat(path)
		if err != nil {
			a.Console.Say(AIWarning, "  File not found (already removed?): "+file, AILine)

			continue
		}

		// A link is removed as a link, whatever it points at
		ok := false

		switch {
		case info.IsDir():
			if ok = os.RemoveAll(path) == nil; ok {
				a.say(AIInfo, "  Removed directory: "+file)
			} else {
				a.Console.Say(AIError, "  Failed to remove directory: "+file, AILine)
			}
		default:
			if ok = os.Remove(path) == nil; ok {
				a.say(AIInfo, "  Removed file: "+file)
			} else {
				a.Console.Say(AIError, "  Failed to remove file: "+file, AILine)
			}
		}

		if ok {
			removed++

			continue
		}

		// Still on disk, so still the tool's
		entries, _ := registration.Get(kindOf[file])
		entry, _ := entries.(*core.AIObject).Get(file)
		keptEntries, _ := kept.Get(kindOf[file])
		keptEntries.(*core.AIObject).Set(file, entry)
	}

	a.say(AIInfo, "Clearing registration...")

	if !a.saveRegistration(kept) {
		a.Console.Say(AIError, "Failed to clear registration file", AILine)

		return 1
	}

	if notRemoved := length(mustGet(kept, "skills")) + length(mustGet(kept, "agents")); notRemoved > 0 {
		a.Console.Say(AIWarning, fmt.Sprintf("Registration kept for %d items that were not removed", notRemoved), AILine)
		a.say(AIInfo, fmt.Sprintf("Removed %d items", removed))

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

func mustGet(object *core.AIObject, key string) any {
	value, _ := object.Get(key)

	return value
}

// removable is whether a registered path is one ai-reset may remove: an entry directly inside the
// project's own directory for that kind on some platform of the catalog, or a legacy one, reached without leaving
// the project through a link on the way there. The entry itself may be a link: that is removed,
// not what it points at.
func (a *AITools) removable(kind, path string) bool {
	if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.Contains(path, "\x00") {
		return false
	}

	name, directory := filepath.Base(path), filepath.Dir(path)
	if name == "." || name == ".." || name == "/" {
		return false
	}

	known := false

	catalog := a.catalog()

	// What an earlier version put where a platform no longer looks is still the tool's to remove
	candidates := catalog.Directories(kind)
	for _, legacy := range catalog.LegacyDirectories(kind) {
		candidates = append(candidates, legacy.Directory)
	}

	for _, candidate := range candidates {
		if filepath.Clean(candidate) == directory {
			known = true

			break
		}
	}

	if !known {
		return false
	}

	for _, part := range strings.Split(directory, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}

	root, err := filepath.EvalSymlinks(a.Dir)
	if err != nil {
		return false
	}

	resolved, err := filepath.EvalSymlinks(a.path(directory))
	if err != nil {
		// Nothing there to remove: not found is said later, and nothing leaves the project
		return errors.Is(err, os.ErrNotExist) && !a.linkOnTheWay(directory)
	}

	return resolved == filepath.Join(root, directory)
}

// linkOnTheWay is whether some existing part of a relative directory is a link.
func (a *AITools) linkOnTheWay(directory string) bool {
	current := ""

	for _, part := range strings.Split(directory, "/") {
		current = filepath.Join(current, part)

		info, err := os.Lstat(a.path(current))
		if err != nil {
			return false
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}

	return false
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
	if !a.hasData("ai-platforms.json") {
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
