package dockergento

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// ~/.hm/config.json is created with the defaults the first time install asks for it, and what is
// saved is what is read next — outside the installation, which is one binary with nowhere to
// write beside it, and shared by every project.
func TestInstallSettingsLiveOutsideTheInstallation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hm", "config.json")

	engine := New(Options{InstallSettingsFile: file, Assets: fstest.MapFS{}})

	settings, err := engine.InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	written, _ := os.ReadFile(file)
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

	if err := os.WriteFile(file, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := engine.InstallSettings(); err == nil {
		t.Error("a broken config.json must not read")
	}
}

// Empty is ~/.hm/config.json, beside the rest of what this machine keeps.
func TestInstallSettingsDefaultToTheHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := New(Options{Assets: fstest.MapFS{}}).InstallSettings(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, ".hm", "config.json")); err != nil {
		t.Errorf("the settings were not created in ~/.hm: %v", err)
	}
}

// The table of equivalent releases is the tool's own.
func TestEquivalentsAreReadFromTheDataDirectory(t *testing.T) {
	assets := fstest.MapFS{"data/equivalent_versions.json": {Data: []byte(`{"2.4.7-p1": "2.4.7"}`)}}

	table, err := New(Options{Assets: assets}).Equivalents()
	if err != nil || table["2.4.7-p1"] != "2.4.7" {
		t.Errorf("Equivalents = %v, %v", table, err)
	}
}
