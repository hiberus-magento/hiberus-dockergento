package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Pointing the store at this machine: what `hm config-env` does on its own, and what an import does
// after it. The settings are the project's config/docker/local_settings.json, seeded from the
// tool's the first time; every entry is one `bin/magento <command>` in the php container, with URL
// and DOMAIN replaced by this project's.
//

// execRecorder is an Orchestrator that only records what it was asked to run.
type execRecorder struct {
	orchestrator
	ran [][]string
}

func (e *execRecorder) Exec(_ core.Project, _ core.ComposeFiles, service string, command []string,
	_ core.ExecOptions) (int, error) {
	e.ran = append(e.ran, append([]string{service}, command...))

	return 0, e.err
}

const shippedSettings = `{
  "config_set": [
    {"path": "web/secure/base_url", "value": "URL"},
    {"path": "web/cookie/cookie_domain", "value": "DOMAIN", "scope": "websites", "scope-code": "base"}
  ]
}`

func settingsLab(t *testing.T) (root, shipped string) {
	t.Helper()

	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}

	shipped = filepath.Join(t.TempDir(), "local_settings.json")
	if err := os.WriteFile(shipped, []byte(shippedSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	return root, shipped
}

func TestDevelopmentSettingsRunOneMagentoCommandPerEntry(t *testing.T) {
	root, shipped := settingsLab(t)
	recorder := &execRecorder{}
	importer := Importer{Orchestrator: recorder, Settings: shipped}

	err := importer.ConfigureForDevelopment(core.Project{Root: root, Domain: "shop.test"}, core.ComposeFiles{})
	if err != nil {
		t.Fatalf("ConfigureForDevelopment = %v, want no error", err)
	}

	want := []string{
		"phpfpm php ./bin/magento config:set web/secure/base_url https://shop.test/",
		"phpfpm php ./bin/magento config:set --scope=websites --scope-code=base web/cookie/cookie_domain shop.test",
	}

	if len(recorder.ran) != len(want) {
		t.Fatalf("ran %v, want %v", recorder.ran, want)
	}

	for i := range want {
		if got := strings.Join(recorder.ran[i], " "); got != want[i] {
			t.Errorf("command %d = %q, want %q", i, got, want[i])
		}
	}

	// The project's own copy is seeded from the tool's, so it can be edited from then on
	if _, err := os.Stat(filepath.Join(root, "config", "docker", "local_settings.json")); err != nil {
		t.Errorf("the project's settings were not seeded: %v", err)
	}
}

// TestDevelopmentSettingsNeedADomain: with no domain there is nothing to point the store at, and
// configuring it with an empty address is worse than leaving it alone. The shell implementation
// asked for one; the binary says so and runs nothing.
func TestDevelopmentSettingsNeedADomain(t *testing.T) {
	root, shipped := settingsLab(t)
	recorder := &execRecorder{}
	importer := Importer{Orchestrator: recorder, Settings: shipped, Binary: "hm"}

	err := importer.ConfigureForDevelopment(core.Project{Root: root}, core.ComposeFiles{})

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "no_domain" || refusal.Code != 2 {
		t.Fatalf("ConfigureForDevelopment with no domain = %v, want a no_domain refusal with code 2", err)
	}

	if len(recorder.ran) != 0 {
		t.Errorf("ran %v, want nothing", recorder.ran)
	}
}
