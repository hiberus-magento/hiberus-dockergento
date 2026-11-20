package dockergento

import (
	"os"
	"path/filepath"
	"strings"
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

// A 1.x installation kept the settings in ~/hm/data/config.json. The first time hm 2 asks for
// ~/.hm/config.json and it is missing, a valid 1.x file is copied once and said so; an invalid
// one is not, and an existing ~/.hm/config.json is never overwritten.
func TestTheSettingsOfA1xInstallationAreCopiedOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, "hm", "data", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}

	old := core.RenderInstallSettings(core.SetInstallSetting(core.DefaultInstallSettings(), "currency", "USD"))
	if err := os.WriteFile(legacy, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	said := ""
	engine := New(Options{Assets: fstest.MapFS{}, Announce: func(message string) { said += message }})

	settings, err := engine.InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	if value, _ := core.InstallSetting(settings, "currency"); value != "USD" {
		t.Errorf("currency = %q, want the 1.x USD", value)
	}

	target := filepath.Join(home, ".hm", "config.json")
	if written, _ := os.ReadFile(target); string(written) != old {
		t.Errorf("~/.hm/config.json = %q, want the 1.x file", written)
	}

	if !strings.Contains(said, legacy) || !strings.Contains(said, target) {
		t.Errorf("announced %q, want both paths", said)
	}

	// Never again, and never over what is there now
	if err := os.WriteFile(legacy, []byte(core.RenderInstallSettings(core.DefaultInstallSettings())), 0o644); err != nil {
		t.Fatal(err)
	}

	again, _ := engine.InstallSettings()
	if value, _ := core.InstallSetting(again, "currency"); value != "USD" {
		t.Errorf("currency = %q after the 1.x file changed, want ~/.hm's USD", value)
	}
}

func TestAnInvalid1xSettingsFileIsNotCopied(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, "hm", "data", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	settings, err := New(Options{Assets: fstest.MapFS{}}).InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	if core.RenderInstallSettings(settings) != core.RenderInstallSettings(core.DefaultInstallSettings()) {
		t.Errorf("settings = %v, want the defaults", settings)
	}
}

// The copy can hold the admin password, so it is readable by this user only; and a 1.x
// config.json that is a link is not followed wherever it points.
func TestThe1xSettingsCopyIsPrivateAndNeverFollowsALink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, "hm", "data", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}

	old := core.RenderInstallSettings(core.SetInstallSetting(core.DefaultInstallSettings(), "currency", "USD"))
	if err := os.WriteFile(legacy, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(home, ".hm", "config.json")

	if _, err := New(Options{Assets: fstest.MapFS{}}).InstallSettings(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("~/.hm/config.json mode = %o, want 600", mode)
	}

	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}

	elsewhere := filepath.Join(t.TempDir(), "config.json")
	if err := os.Rename(legacy, elsewhere); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(elsewhere, legacy); err != nil {
		t.Fatal(err)
	}

	settings, err := New(Options{Assets: fstest.MapFS{}}).InstallSettings()
	if err != nil {
		t.Fatal(err)
	}

	if value, _ := core.InstallSetting(settings, "currency"); value == "USD" {
		t.Error("the settings were read through a link out of the 1.x installation")
	}
}

// What is copied is what the installer copies: one valid JSON object, nothing else.
func TestOnlyAValidJSONObjectIs1xSettings(t *testing.T) {
	for _, contents := range []string{`{"currency": }`, `{"a": 1}{"b": 2}`, `{"a": 1,}`, `[1]`, `"text"`, ``} {
		home := t.TempDir()
		t.Setenv("HOME", home)

		legacy := filepath.Join(home, "hm", "data", "config.json")
		if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(legacy, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}

		said := ""
		if _, err := New(Options{Assets: fstest.MapFS{}, Announce: func(message string) { said += message }}).InstallSettings(); err != nil {
			t.Fatal(err)
		}

		if said != "" {
			t.Errorf("%q was copied: %s", contents, said)
		}
	}
}

// The settings can hold the store's admin password, so the file is the user's alone: written
// 0600, and narrowed to it when an older version left it readable by everyone.
func TestInstallSettingsAreReadableOnlyByTheirOwner(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	engine := New(Options{InstallSettingsFile: file, Assets: fstest.MapFS{}})

	for _, before := range []os.FileMode{0, 0o644} {
		if before != 0 {
			if err := os.WriteFile(file, []byte("{}\n"), before); err != nil {
				t.Fatal(err)
			}
			os.Chmod(file, before) //nolint:errcheck
		}

		if err := engine.SaveInstallSettings([]core.Setting{{Key: "admin-password", Value: []byte(`"secret"`)}}); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("starting from %v the file is %v, want 0600", before, mode)
		}

		os.Remove(file) //nolint:errcheck
	}
}

// A config.json that is a link — dotfiles kept somewhere else — stays a link: the settings are
// written where it points, not over the link itself.
func TestInstallSettingsWriteThroughALink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "hm-config.json")
	link := filepath.Join(dir, "config.json")

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	engine := New(Options{InstallSettingsFile: link, Assets: fstest.MapFS{}})
	if err := engine.SaveInstallSettings([]core.Setting{{Key: "currency", Value: []byte(`"EUR"`)}}); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config.json is no longer a link: %v, %v", info, err)
	}

	written, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(written), "EUR") {
		t.Errorf("the link's target holds %q, %v; want the saved settings", written, err)
	}
}

// A link whose target does not exist yet — dotfiles cloned, the file not created — is written
// through too: the target is created and the link kept.
func TestInstallSettingsCreateADanglingLinksTarget(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "config.json")

	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("dotfiles", "hm-config.json"), link); err != nil {
		t.Fatal(err)
	}

	engine := New(Options{InstallSettingsFile: link, Assets: fstest.MapFS{}})
	if err := engine.SaveInstallSettings([]core.Setting{{Key: "currency", Value: []byte(`"EUR"`)}}); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config.json is no longer a link: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(dir, "dotfiles", "hm-config.json"))
	if err != nil || !strings.Contains(string(written), "EUR") {
		t.Errorf("the target holds %q, %v; want the saved settings", written, err)
	}
}
