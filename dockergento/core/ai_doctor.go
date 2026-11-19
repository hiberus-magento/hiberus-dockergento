package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

//
// What ai-doctor reads, and the rules it applies to what it finds. ai-pull, ai-reset and ai-init
// read the same files, which is why these are here and not inside the command.
//

// AIPropertiesFile and AIRegistrationFile are the two files of a project's AI tooling: what it asked
// for, and what the tool installed. The registration is read from where the command runs, as
// console/tasks/ai_registration.sh read it.
const (
	AIPropertiesFile   = "config/docker/ai-properties.json"
	AIRegistrationFile = "config/docker/ai-registration.json"
)

// ErrAIRegistrationCorrupt is a registration that is not JSON: nothing can be said about what was
// installed, and nothing that writes it may go on.
var ErrAIRegistrationCorrupt = errors.New("invalid JSON in " + AIRegistrationFile)

// The states a resource can be in.
const (
	// AIStateCustom is a resource the tool did not install, and that ai-pull leaves alone.
	AIStateCustom = "custom"
	// AIStateCurrent is what was installed, unchanged.
	AIStateCurrent = "current"
	// AIStateUnknown was installed before there was a checksum worth recording.
	AIStateUnknown = "unknown"
	// AIStateModified has changed since it was installed: the next ai-pull loses those edits.
	AIStateModified = "modified"
	// AIStateOutdated is unchanged, and behind the copy that came with the tool.
	AIStateOutdated = "outdated"
	// AIStateMissing is tracked, and no longer there.
	AIStateMissing = "missing"
)

// AIResource is one installed skill or agent, as ai-doctor reports it. A field with nothing to say
// is "-", as the script printed it.
type AIResource struct {
	Platform  string `json:"platform"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Origin    string `json:"origin"`
	Version   string `json:"version"`
	Installed string `json:"installed"`
}

// AIDoctorReport is what ai-doctor found. Resources is never nil, so the document says [] and not
// null.
type AIDoctorReport struct {
	Configured bool         `json:"configured"`
	Resources  []AIResource `json:"resources"`
}

// AIRegistrationEntry is what the installer recorded for one resource, with the script's defaults
// for what it did not record: no checksum is "", anything else is "-", and only the date of the
// installation is kept.
type AIRegistrationEntry struct {
	Checksum  string
	Origin    string
	Version   string
	Installed string
}

// AIRegistration is the parsed config/docker/ai-registration.json.
type AIRegistration struct {
	document map[string]any
}

// ReadAIRegistration parses a registration. Contents that are not JSON are ErrAIRegistrationCorrupt;
// nothing at all is an empty registration.
func ReadAIRegistration(contents []byte) (AIRegistration, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		return AIRegistration{}, nil
	}

	var document any
	if err := json.Unmarshal(contents, &document); err != nil {
		return AIRegistration{}, ErrAIRegistrationCorrupt
	}

	object, _ := document.(map[string]any)

	return AIRegistration{document: object}, nil
}

// Entry is what was recorded for a resource of a kind. The installer records the path it was
// given, which is usually the one relative to the project, so an entry is looked up by its
// absolute path first and its relative one after (`.[$k][$absolute] // .[$k][$relative]`). An
// entry that records nothing is not found: the resource is custom.
func (r AIRegistration) Entry(kind, absolute, relative string) (AIRegistrationEntry, bool) {
	entries, _ := r.document[kind].(map[string]any)

	value := entries[absolute]
	if !present(value) {
		value = entries[relative]
	}

	fields, ok := value.(map[string]any)
	if !ok || len(fields) == 0 {
		return AIRegistrationEntry{}, false
	}

	installed := "-"
	if present(fields["installed"]) {
		installed = ""
		if text, ok := fields["installed"].(string); ok {
			installed = text
			if runes := []rune(text); len(runes) > 10 {
				installed = string(runes[:10])
			}
		}
	}

	return AIRegistrationEntry{
		Checksum:  jqText(fields["checksum"], ""),
		Origin:    orDash(jqText(fields["origin"], "")),
		Version:   orDash(jqText(fields["version"], "")),
		Installed: installed,
	}, true
}

// Tracked is every path recorded for a kind, in key order (`keys[]`).
func (r AIRegistration) Tracked(kind string) []string {
	entries, _ := r.document[kind].(map[string]any)

	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	return paths
}

// AIResourceState is the state of an installed resource: what was recorded against what is there
// now and, for what came with the tool, against the copy it shipped. That copy only matters for a
// resource that is otherwise current — an edited one is modified whatever the tool has.
func AIResourceState(recorded, current string, shipped *string) string {
	switch {
	case recorded == "":
		return AIStateUnknown
	case recorded != current:
		return AIStateModified
	case shipped != nil && *shipped != current:
		return AIStateOutdated
	default:
		return AIStateCurrent
	}
}

// ReadAIPlatforms is the platforms ai-properties.json asks for, split into words as the script's
// `for platform in $platforms` split them. Contents that cannot be read ask for none.
func ReadAIPlatforms(contents []byte) []string {
	var properties struct {
		Platforms []any `json:"platforms"`
	}

	if json.Unmarshal(contents, &properties) != nil {
		return nil
	}

	names := make([]string, 0, len(properties.Platforms))
	for _, platform := range properties.Platforms {
		names = append(names, jqText(platform, ""))
	}

	if words := strings.Fields(strings.Join(names, " ")); len(words) > 0 {
		return words
	}

	return nil
}

// AIPlatformCatalog is data/ai-platforms.json: where each platform keeps its skills and agents.
type AIPlatformCatalog struct {
	platforms map[string]map[string]any
}

// ReadAIPlatformCatalog parses the catalog. Contents that cannot be read know no platform.
func ReadAIPlatformCatalog(contents []byte) AIPlatformCatalog {
	var catalog struct {
		Platforms map[string]map[string]any `json:"platforms"`
	}

	_ = json.Unmarshal(contents, &catalog)

	return AIPlatformCatalog{platforms: catalog.Platforms}
}

// Directory is where a platform keeps a kind ("skills" or "agents"), relative to the project, or ""
// when it keeps none.
func (c AIPlatformCatalog) Directory(platform, kind string) string {
	value := c.platforms[platform][kind+"_dir"]
	if !present(value) {
		return ""
	}

	return jqText(value, "")
}

// present is what jq's `//` keeps: anything but null and false.
func present(value any) bool {
	if value == nil {
		return false
	}

	if flag, ok := value.(bool); ok && !flag {
		return false
	}

	return true
}

// jqText is a value as `jq -r '. // fallback'` prints it.
func jqText(value any, fallback string) string {
	if !present(value) {
		return fallback
	}

	if text, ok := value.(string); ok {
		return text
	}

	encoded, _ := json.Marshal(value)

	return string(encoded)
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}

	return value
}
