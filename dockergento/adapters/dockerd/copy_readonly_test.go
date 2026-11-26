package dockerd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//
// A file on the host that is read-only is replaced, as `docker cp` replaces it: git keeps its
// objects 0444, and a package Composer installed from source carries them under vendor.
//

func TestUnpackingReplacesAReadOnlyFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "shop")
	object := filepath.Join(target, "vendor", "acme", "pkg", ".git", "objects", "00", "1c2e")

	if err := os.MkdirAll(filepath.Dir(object), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(object, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}

	stream := archived(t, [][2]string{{"./vendor/acme/pkg/.git/objects/00/1c2e", "new"}})
	if err := unpack(stream, target, ".", nil); err != nil {
		t.Fatalf("unpack over a read-only file: %v", err)
	}

	if got, _ := os.ReadFile(object); string(got) != "new" {
		t.Fatalf("the read-only file holds %q, want it replaced", got)
	}
}

func TestAFileThatStillCannotBeWrittenNamesThePathAndWhy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through any permission")
	}

	target := filepath.Join(t.TempDir(), "shop")
	locked := filepath.Join(target, "locked")
	file := filepath.Join(locked, "one.php")

	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) }) //nolint:errcheck

	err := unpack(archived(t, [][2]string{{"./locked/one.php", "new"}}), target, ".", nil)
	if err == nil {
		t.Fatal("unpack into a read-only directory succeeded, want it refused")
	}

	for _, wanted := range []string{file, "permission denied"} {
		if !strings.Contains(err.Error(), wanted) {
			t.Errorf("error %q, want it to name %q", err, wanted)
		}
	}

	if got, _ := os.ReadFile(file); string(got) != "old" {
		t.Fatalf("the file holds %q, want it as it was", got)
	}
}
