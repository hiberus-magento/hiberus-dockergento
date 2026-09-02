// Package fsprops reads a project's properties from config/docker/properties.json.
package fsprops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Reader loads properties from disk.
type Reader struct {
	// Defaults is the tool's own properties file, which the project's is merged over. Without it
	// a project that never set WORKDIR_PHP or DOCKER_COMPOSE_FILE has no value for them, and the
	// answer silently differs from the shell implementation's — which merges the two.
	Defaults string
}

// Load returns the properties of the project rooted at dir, merged over the defaults the tool
// ships. A directory with no properties file returns an empty map and no error: it is a
// directory that is not a project, which several commands are perfectly entitled to be in.
//
// Values are read as strings because that is what they are in the file — and what the shell
// implementation assumes. A boolean there breaks its `jq` outright, which is a compatibility
// constraint rather than a preference.
func (r Reader) Load(dir string) (map[string]string, error) {
	properties := map[string]string{}

	// The tool's defaults first, so the project's file only has to say what it changes
	if r.Defaults != "" {
		defaults, err := read(r.Defaults)
		if err != nil {
			return nil, err
		}

		for key, value := range defaults {
			properties[key] = value
		}
	}

	own, err := read(filepath.Join(dir, "config", "docker", "properties.json"))
	if err != nil {
		return nil, err
	}

	for key, value := range own {
		properties[key] = value
	}

	// A directory with no properties of its own is a directory that is not a project, and the
	// caller decides what that means — but it must not look configured just because the defaults
	// exist
	if len(own) == 0 {
		return map[string]string{}, nil
	}

	return properties, nil
}

func read(file string) (map[string]string, error) {
	contents, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(contents, &raw); err != nil {
		return nil, err
	}

	properties := make(map[string]string, len(raw))
	for key, value := range raw {
		if text, ok := value.(string); ok {
			properties[key] = text
		}
	}

	return properties, nil
}

// Set records one property, leaving the rest of the file alone.
//
// Merged and not rebuilt: the file is committed, so rebuilding it from the keys somebody happened
// to be thinking about is how a project loses a setting nobody was looking at — which is exactly
// what happened when a domain read from a database was written through the file format this tool
// used to have.
func (r Reader) Set(dir, key, value string) error {
	file := filepath.Join(dir, "config", "docker", "properties.json")

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}

	own := map[string]any{}

	if contents, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(contents, &own); err != nil {
			return fmt.Errorf("%s is not valid JSON: %w", file, err)
		}
	}

	own[key] = value

	document, err := json.MarshalIndent(own, "", "  ")
	if err != nil {
		return err
	}

	// Written beside and moved into place: a half-written properties.json is a project with no
	// identity, and this runs while somebody is importing a database
	temporary := file + ".tmp"

	if err := os.WriteFile(temporary, append(document, '\n'), 0o644); err != nil { //nolint:gosec
		return err
	}

	return os.Rename(temporary, file)
}
