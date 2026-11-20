package dockergento

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	hm "github.com/hiberus-magento/hiberus-dockergento"
)

// Every reader of the tool's own files goes through Options.Assets: an engine handed a MapFS
// answers from it and from nothing on disk.
func TestTheEngineReadsTheToolsFilesFromTheInjectedAssets(t *testing.T) {
	assets := fstest.MapFS{
		"data/equivalent_versions.json":   {Data: []byte(`{"2.4.7-p9": "2.4.7"}`)},
		"data/requirements.json":          {Data: []byte(`{"2.4.7": {"php": "8.2-fpm"}}`)},
		"data/default_files_magento.json": {Data: []byte(`{"zeta": true, "alpha": true}`)},
	}

	engine := New(Options{Root: t.TempDir(), Assets: assets})

	table, err := engine.Equivalents()
	if err != nil || table["2.4.7-p9"] != "2.4.7" {
		t.Errorf("Equivalents = %v, %v", table, err)
	}

	if _, err := engine.Compatibility(); err != nil {
		t.Errorf("Compatibility = %v, want the table from the injected assets", err)
	}

	files, err := engine.DefaultMagentoFiles()
	if err != nil || len(files) != 2 || files[0] != "alpha" {
		t.Errorf("DefaultMagentoFiles = %v, %v", files, err)
	}

	if engine.Assets() == nil {
		t.Error("the engine does not say what it reads")
	}
}

// The defaults a project's properties are merged over are the tool's, from the same assets.
func TestThePropertyDefaultsComeFromTheInjectedAssets(t *testing.T) {
	assets := fstest.MapFS{"data/properties.json": {Data: []byte(`{"WORKDIR_PHP": "/from/assets"}`)}}
	project := proyecto(t, `{"COMPOSE_PROJECT_NAME": "tienda"}`)

	values, err := New(Options{Assets: assets}).properties().Load(project.Root)
	if err != nil {
		t.Fatal(err)
	}

	if values["WORKDIR_PHP"] != "/from/assets" || values["COMPOSE_PROJECT_NAME"] != "tienda" {
		t.Errorf("properties = %v, want the project's over the assets' defaults", values)
	}
}

// Nothing set is the copy compiled into the binary, whatever is or is not beside the executable.
func TestWithNoAssetsTheEngineReadsTheEmbeddedCopy(t *testing.T) {
	t.Setenv(hm.AssetsDirVariable, "")

	if _, err := New(Options{Root: t.TempDir()}).Compatibility(); err != nil {
		t.Errorf("Compatibility from the embedded copy = %v", err)
	}
}

// HM_ASSETS_DIR is a contributor's checkout read in place of the embedded copy.
func TestTheAssetsDirectoryOverridesTheEmbeddedCopy(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "data", "equivalent_versions.json"), []byte(`{"9.9.9": "2.4.7"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(hm.AssetsDirVariable, dir)

	table, err := New(Options{}).Equivalents()
	if err != nil || len(table) != 1 || table["9.9.9"] != "2.4.7" {
		t.Errorf("Equivalents = %v, %v; want the override's table", table, err)
	}
}

// The permissions an agent is given come from the tool's own files, which are the assets: there
// is no data/ beside a release binary to read them from.
func TestAgentPermissionsAreReadFromTheAssets(t *testing.T) {
	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{
		"data/command_descriptions.json": {Data: []byte(`{"status": {"safety": "safe"}}`)},
		"data/ai-exclusions.json":        {Data: []byte(`{}`)},
	}})

	if _, err := engine.AgentPermissions(false); err != nil {
		t.Fatalf("AgentPermissions = %v, want it read from the assets", err)
	}

	empty := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}})
	if _, err := empty.AgentPermissions(false); err == nil || !strings.Contains(err.Error(), "command_descriptions.json") {
		t.Fatalf("AgentPermissions with no assets = %v, want the missing file named", err)
	}
}
