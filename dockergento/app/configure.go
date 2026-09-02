package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Pointing a store that came from somewhere else at this machine.
//
// A dump carries the addresses, the search engine and the security settings of wherever it was
// taken. Left alone, the store redirects to production the first time somebody opens it — which
// is the single most alarming thing this tool could let happen quietly.
//
// What to change lives in local_settings.json, in the project, so that a project can add to it.
// The tool ships one and copies it in the first time.
//

const settingsFile = "config/docker/local_settings.json"

// setting is one thing to set, with the scope it applies to when it has one.
type setting struct {
	Path      string `json:"path"`
	Value     string `json:"value"`
	Scope     string `json:"scope,omitempty"`
	ScopeCode string `json:"scope-code,omitempty"`
}

// configure runs the settings through Magento's own CLI.
func (d Importer) configure(project core.Project, files core.ComposeFiles) error {
	domain := project.Domain
	if domain == "" {
		// Nothing to point anywhere: the store would be configured with an empty address, which
		// is worse than leaving it as it came
		return nil
	}

	settings, err := d.settings(project)
	if err != nil {
		return err
	}

	for command, entries := range settings {
		// `config_set` is how the file names `config:set`
		magento := strings.Replace(command, "_", ":", 1)

		for _, entry := range entries {
			arguments := []string{"php", "./bin/magento", magento}

			if entry.Scope != "" {
				arguments = append(arguments, "--scope="+entry.Scope, "--scope-code="+entry.ScopeCode)
			}

			arguments = append(arguments, entry.Path, valueFor(entry.Value, domain))

			d.announce("🚀 bin/magento " + strings.Join(arguments[2:], " ") + "\n")

			//
			// A failure here is not a failure of the import: the data is in. What is not done is
			// pointing the store at this machine, and a store still carrying the addresses of
			// wherever the dump was taken will redirect to production the first time somebody
			// opens it. So it is said, rather than passed on as whatever Docker happened to
			// return.
			//
			if _, err := d.Orchestrator.Exec(project, files, "phpfpm", arguments,
				core.ExecOptions{Interactive: true}); err != nil {
				return core.Refusal{
					Kind: "not_configured",
					Code: 1,
					Message: fmt.Sprintf(
						"The database was imported, but the store was not pointed at this machine: %s", err),
					Hint: d.Binary + " start   # then run the import again, or set the URLs by hand",
				}
			}
		}
	}

	return nil
}

// valueFor fills in the two placeholders the file uses, which are the two things it cannot know.
func valueFor(value, domain string) string {
	switch value {
	case "URL":
		return "https://" + domain + "/"
	case "DOMAIN":
		return domain
	}

	return value
}

// settings are the project's, seeded from the tool's the first time.
func (d Importer) settings(project core.Project) (map[string][]setting, error) {
	own := filepath.Join(project.Root, settingsFile)

	if _, err := os.Stat(own); err != nil && d.Settings != "" {
		shipped, err := os.ReadFile(d.Settings)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(own, shipped, 0o644); err != nil { //nolint:gosec
			return nil, err
		}
	}

	contents, err := os.ReadFile(own)
	if err != nil {
		return nil, fmt.Errorf("no settings to apply: %w", err)
	}

	var settings map[string][]setting
	if err := json.Unmarshal(contents, &settings); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", own, err)
	}

	return settings, nil
}
