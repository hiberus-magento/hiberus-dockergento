// Package fsprops reads a project's properties from config/docker/properties.json.
package fsprops

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Reader loads properties from disk.
type Reader struct{}

// Load returns the properties of the project rooted at dir, merged over the defaults the tool
// ships. A directory with no properties file returns an empty map and no error: it is a
// directory that is not a project, which several commands are perfectly entitled to be in.
//
// Values are read as strings because that is what they are in the file — and what the shell
// implementation assumes. A boolean there breaks its `jq` outright, which is a compatibility
// constraint rather than a preference.
func (Reader) Load(dir string) (map[string]string, error) {
	file := filepath.Join(dir, "config", "docker", "properties.json")

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
