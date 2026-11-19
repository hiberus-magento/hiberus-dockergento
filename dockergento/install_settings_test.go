package dockergento

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// data/config.json is created with the defaults the first time install asks for it, and what is
// saved is what is read next — in the tool's data directory, shared by every project.
func TestInstallSettingsLiveInTheToolsDataDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: root})

	settings, err := engine.InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	written, _ := os.ReadFile(filepath.Join(root, "data", "config.json"))
	if string(written) != core.RenderInstallSettings(core.DefaultInstallSettings()) {
		t.Fatalf("created %q", written)
	}

	if err := engine.SaveInstallSettings(core.SetInstallSetting(settings, "currency", "USD")); err != nil {
		t.Fatal(err)
	}

	again, err := engine.InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	if value, _ := core.InstallSetting(again, "currency"); value != "USD" {
		t.Errorf("currency = %q after saving USD", value)
	}

	if err := os.WriteFile(filepath.Join(root, "data", "config.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := engine.InstallSettings(); err == nil {
		t.Error("a broken data/config.json must not read")
	}
}

// The table of equivalent releases is the tool's own.
func TestEquivalentsAreReadFromTheDataDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "data", "equivalent_versions.json"), []byte(`{"2.4.7-p1": "2.4.7"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	table, err := New(Options{Root: root}).Equivalents()
	if err != nil || table["2.4.7-p1"] != "2.4.7" {
		t.Errorf("Equivalents = %v, %v", table, err)
	}
}
