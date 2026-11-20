package dockergento

import (
	"testing"
	"testing/fstest"
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
