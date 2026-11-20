package dockergento

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The two things Composer's macOS dance asks of this machine rather than of a container: which
// files a Magento project ships at its root, and emptying the host's vendor before the workdir is
// copied back over it. The second deletes, so where it may delete is what is pinned hardest.
//

func TestDefaultMagentoFilesAreTheKeysOfTheToolsOwnList(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "data", "default_files_magento.json"))
	if err != nil {
		t.Fatal(err)
	}

	listed := map[string]bool{}
	if err := json.Unmarshal(contents, &listed); err != nil {
		t.Fatal(err)
	}

	want := []string{}
	for key := range listed {
		want = append(want, key)
	}

	sort.Strings(want)

	got, err := New(Options{Assets: os.DirFS(root)}).DefaultMagentoFiles()
	if err != nil {
		t.Fatalf("DefaultMagentoFiles() = %v, want no error", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("DefaultMagentoFiles() (-want +got):\n%s", diff)
	}
}

func TestDefaultMagentoFilesWithoutTheListIsAnError(t *testing.T) {
	if _, err := motor(t).DefaultMagentoFiles(); err == nil {
		t.Fatal("DefaultMagentoFiles() from a tool with no data directory = nil, want an error")
	}
}

// write makes a file, and the directories it is in.
func write(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

func TestClearVendorEmptiesItAndKeepsIt(t *testing.T) {
	project := proyecto(t, `{"MAGENTO_DIR": "./src", "COMPOSE_PROJECT_NAME": "tienda"}`)
	vendor := filepath.Join(project.Root, "src", "vendor")

	write(t, filepath.Join(vendor, "magento", "framework", "a.php"))
	write(t, filepath.Join(vendor, "autoload.php"))
	write(t, filepath.Join(vendor, ".hidden"))
	write(t, filepath.Join(project.Root, "src", "composer.json"))
	write(t, filepath.Join(project.Root, "src", "app", "etc", "env.php"))

	if err := motor(t).ClearVendor(project.Root); err != nil {
		t.Fatalf("ClearVendor() = %v, want no error", err)
	}

	if info, err := os.Stat(vendor); err != nil || !info.IsDir() {
		t.Fatalf("vendor is gone (%v), want the directory kept", err)
	}

	for _, gone := range []string{"magento", "autoload.php"} {
		if exists(filepath.Join(vendor, gone)) {
			t.Errorf("vendor/%s is still there, want it deleted", gone)
		}
	}

	// rm -rf vendor/* never matched a name that starts with a dot
	if !exists(filepath.Join(vendor, ".hidden")) {
		t.Error("vendor/.hidden was deleted, and the shell's glob never matched it")
	}

	for _, kept := range []string{"composer.json", filepath.Join("app", "etc", "env.php")} {
		if !exists(filepath.Join(project.Root, "src", kept)) {
			t.Errorf("src/%s was deleted, want everything beside vendor left alone", kept)
		}
	}
}

func TestClearVendorWithNoVendorHasNothingToDo(t *testing.T) {
	project := proyecto(t, `{"COMPOSE_PROJECT_NAME": "tienda"}`)

	if err := motor(t).ClearVendor(project.Root); err != nil {
		t.Fatalf("ClearVendor() = %v, want no error", err)
	}
}

func TestClearVendorRefusesAVendorOutsideTheProject(t *testing.T) {
	cases := []struct {
		name       string
		magentoDir string
	}{
		{"the parent directory", ".."},
		{"a sibling of the project", "../elsewhere"},
		{"a climb that comes back somewhere else", "src/../../elsewhere"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			project := proyecto(t, `{"MAGENTO_DIR": "`+tt.magentoDir+`", "COMPOSE_PROJECT_NAME": "tienda"}`)

			// What would be deleted if the path were taken at its word
			victim := filepath.Join(project.Root, tt.magentoDir, "vendor", "precious.txt")
			write(t, victim)

			err := motor(t).ClearVendor(project.Root)

			var refusal core.Refusal
			if !errors.As(err, &refusal) || refusal.Kind != "unsafe_vendor_path" {
				t.Fatalf("ClearVendor() = %v, want the unsafe_vendor_path refusal", err)
			}

			if !exists(victim) {
				t.Fatalf("%s was deleted, want nothing outside the project touched", victim)
			}
		})
	}
}

func TestClearVendorRefusesAVendorThatIsALink(t *testing.T) {
	project := proyecto(t, `{"COMPOSE_PROJECT_NAME": "tienda"}`)

	elsewhere := t.TempDir()
	victim := filepath.Join(elsewhere, "precious.txt")
	write(t, victim)

	if err := os.Symlink(elsewhere, filepath.Join(project.Root, "vendor")); err != nil {
		t.Fatal(err)
	}

	err := motor(t).ClearVendor(project.Root)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "unsafe_vendor_path" {
		t.Fatalf("ClearVendor() = %v, want the unsafe_vendor_path refusal", err)
	}

	if !exists(victim) {
		t.Fatal("what the link points to was deleted, want it left alone")
	}
}

// Even pointing inside the project: what it points to is not <MAGENTO_DIR>/vendor, and that is
// the only directory this may empty.
func TestClearVendorRefusesAVendorThatIsALinkWithinTheProject(t *testing.T) {
	project := proyecto(t, `{"COMPOSE_PROJECT_NAME": "tienda"}`)

	victim := filepath.Join(project.Root, "modules", "vendor", "precious.txt")
	write(t, victim)

	if err := os.Symlink(filepath.Join(project.Root, "modules", "vendor"),
		filepath.Join(project.Root, "vendor")); err != nil {
		t.Fatal(err)
	}

	err := motor(t).ClearVendor(project.Root)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "unsafe_vendor_path" {
		t.Fatalf("ClearVendor() = %v, want the unsafe_vendor_path refusal", err)
	}

	if !exists(victim) {
		t.Fatal("what the link points to was deleted, want it left alone")
	}
}

func TestClearVendorRefusesAMagentoDirThatIsALinkOutOfTheProject(t *testing.T) {
	project := proyecto(t, `{"MAGENTO_DIR": "./src", "COMPOSE_PROJECT_NAME": "tienda"}`)

	elsewhere := t.TempDir()
	victim := filepath.Join(elsewhere, "vendor", "precious.txt")
	write(t, victim)

	if err := os.Symlink(elsewhere, filepath.Join(project.Root, "src")); err != nil {
		t.Fatal(err)
	}

	if err := motor(t).ClearVendor(project.Root); err == nil {
		t.Fatal("ClearVendor() = nil, want a refusal")
	}

	if !exists(victim) {
		t.Fatal("the vendor behind the link was deleted, want it left alone")
	}
}
