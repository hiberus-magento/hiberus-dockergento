// Package fsprops reads a project's properties from config/docker/properties.json.
package fsprops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Reader loads properties from disk.
type Reader struct {
	// Defaults is the tool's own properties file, which the project's is merged over. Without it
	// a project that never set WORKDIR_PHP or DOCKER_COMPOSE_FILE has no value for them, and the
	// answer silently differs from the shell implementation's — which merges the two.
	Defaults string

	// Assets is where Defaults is read from when it is set: the files the tool ships with, in
	// which Defaults is a slash-separated name. Nil means Defaults is a path on disk.
	Assets fs.FS

	// Notices is where converting a project's old properties file is announced. Nil says nothing.
	Notices io.Writer
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
		defaults, err := r.defaults()
		if err != nil {
			return nil, err
		}

		for key, value := range defaults {
			properties[key] = value
		}
	}

	if err := r.convert(dir); err != nil {
		return nil, err
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

// convert turns the format before properties.json — KEY=value lines in config/docker/properties —
// into properties.json, once: only while properties.json does not exist, so a project that has
// both is read exactly as it was. The old file is moved aside rather than deleted, as the shell
// implementation did, because it is committed and somebody may still want to see what it said.
func (r Reader) convert(dir string) error {
	directory := filepath.Join(dir, "config", "docker")
	old := filepath.Join(directory, "properties")
	file := filepath.Join(directory, "properties.json")

	if _, err := os.Stat(file); !os.IsNotExist(err) {
		return nil
	}

	contents, err := os.ReadFile(old)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	document, err := json.MarshalIndent(parseOld(contents), "", "  ")
	if err != nil {
		return err
	}

	temporary := file + ".tmp"

	if err := os.WriteFile(temporary, append(document, '\n'), 0o644); err != nil { //nolint:gosec
		return err
	}

	if err := os.Rename(temporary, file); err != nil {
		return err
	}

	kept := freeName(old + ".bak")

	if err := os.Rename(old, kept); err != nil {
		return fmt.Errorf("converted %s, but could not move it aside: %w", old, err)
	}

	if r.Notices != nil {
		fmt.Fprintf(r.Notices, "Converted config/docker/properties to config/docker/properties.json; "+
			"the old file is kept as config/docker/%s\n", filepath.Base(kept))
	}

	return nil
}

// parseOld reads the old format the way the shell implementation did: blank lines and comments
// skipped, leading spaces trimmed, the key up to the first "=" and the rest the value — which may
// hold more of them — with every double quote removed.
func parseOld(contents []byte) map[string]string {
	properties := map[string]string{}

	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimLeft(line, " \t\r")
		line = strings.TrimRight(line, "\r")

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, _ := strings.Cut(line, "=")
		properties[strings.ReplaceAll(key, `"`, "")] = strings.ReplaceAll(value, `"`, "")
	}

	return properties
}

// freeName is name, or name.1, name.2 … — the first that is not taken.
func freeName(name string) string {
	candidate := name

	for n := 1; ; n++ {
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}

		candidate = name + "." + strconv.Itoa(n)
	}
}

func (r Reader) defaults() (map[string]string, error) {
	if r.Assets == nil {
		return read(r.Defaults)
	}

	contents, err := fs.ReadFile(r.Assets, r.Defaults)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parse(contents)
}

func read(file string) (map[string]string, error) {
	contents, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	return parse(contents)
}

func parse(contents []byte) (map[string]string, error) {
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
