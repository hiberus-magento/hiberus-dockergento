package hm

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// needed is every file the binary reads from its assets, by the name it reads it under. A reader
// that starts asking for another one belongs here too: a file missing from the embedded copy is a
// release that fails on the first machine without a checkout.
var needed = []string{
	"data/ai-exclusions.json",
	"data/ai-platforms.json",
	"data/ai-repositories.json",
	"data/ai-skill-types.json",
	"data/command_descriptions.json",
	"data/default_files_magento.json",
	"data/equivalent_versions.json",
	"data/local_settings.json",
	"data/properties.json",
	"data/requirements.json",
	"docker-compose/docker-compose.template.yml",
	"docker-compose/docker-compose.dev.mac.template.yml",
	"docker-compose/docker-compose.dev.linux.template.yml",
}

func TestTheEmbeddedCopyHasEveryFileTheReadersNeed(t *testing.T) {
	for _, name := range needed {
		contents, err := fs.ReadFile(Embedded(), name)
		if err != nil {
			t.Errorf("%s is not embedded: %v", name, err)

			continue
		}

		if len(contents) == 0 {
			t.Errorf("%s is embedded empty", name)
		}

		if filepath.Ext(name) == ".json" && !json.Valid(contents) {
			t.Errorf("%s is embedded as invalid JSON", name)
		}
	}
}

// The skills ai-pull installs from the tool are the ones in the repository, every one of them.
func TestTheEmbeddedCopyHasTheToolsOwnSkills(t *testing.T) {
	entries, err := os.ReadDir("skills")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("the repository has no skills to compare with")
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if _, err := fs.Stat(Embedded(), "skills/"+entry.Name()+"/SKILL.md"); err != nil {
			t.Errorf("skill %s is not embedded: %v", entry.Name(), err)
		}
	}
}

// A contributor's own install settings are never compiled into a binary.
func TestTheEmbeddedCopyLeavesOutTheInstallSettings(t *testing.T) {
	if _, err := fs.Stat(Embedded(), "data/config.json"); err == nil {
		t.Error("data/config.json is embedded")
	}
}

func TestWithoutTheOverrideTheEmbeddedCopyIsRead(t *testing.T) {
	t.Setenv(AssetsDirVariable, "")

	if Assets() != Embedded() {
		t.Error("Assets() is not the embedded copy")
	}

	if AssetsDir() != "" {
		t.Errorf("AssetsDir() = %q, want empty", AssetsDir())
	}
}

func TestTheOverrideIsReadInsteadOfTheEmbeddedCopy(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "data", "requirements.json"), []byte(`{"edited": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(AssetsDirVariable, dir)

	contents, err := fs.ReadFile(Assets(), "data/requirements.json")
	if err != nil || string(contents) != `{"edited": {}}` {
		t.Errorf("read %q, %v; want the override's file", contents, err)
	}

	// What the directory does not have is not there: no falling back to the embedded copy
	if _, err := fs.Stat(Assets(), "data/properties.json"); err == nil {
		t.Error("a file missing from the override was found anyway")
	}

	if AssetsDir() != dir {
		t.Errorf("AssetsDir() = %q, want %q", AssetsDir(), dir)
	}
}
