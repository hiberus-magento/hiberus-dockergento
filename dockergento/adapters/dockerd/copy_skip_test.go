package dockerd

import (
	"os"
	"path/filepath"
	"testing"
)

// The workdir copied after vendor leaves vendor unwritten: it has just been copied, and writing it
// a second time is the slowest part of the dance done twice.
func TestCopyingTheWorkdirOutCanLeaveNamedEntriesAtItsTopUnwritten(t *testing.T) {
	fake := &daemon{out: archived(t, [][2]string{
		{".", ""}, {"./vendor/autoload.php", "container"}, {"./composer.json", "new"},
		{"./app/vendor/keep.php", "nested"},
	}).Bytes()}
	fake.serve(t)

	target := filepath.Join(t.TempDir(), "shop")
	if err := os.MkdirAll(filepath.Join(target, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(target, "vendor", "autoload.php"), []byte("copied first"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (Files{}).FromSkipping("abc123", "/var/www/html/.", target, []string{"vendor"}, nil); err != nil {
		t.Fatalf("FromSkipping: %v", err)
	}

	for name, want := range map[string]string{
		"vendor/autoload.php": "copied first",
		"composer.json":       "new",
		"app/vendor/keep.php": "nested",
	} {
		if got, _ := os.ReadFile(filepath.Join(target, name)); string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestSkippingCopiesStillLeaveTheRepositoryOut(t *testing.T) {
	target := filepath.Join(t.TempDir(), "shop")

	stream := archived(t, [][2]string{{"./.git/HEAD", "stale"}, {"./composer.json", "new"}})
	if err := unpackSkipping(stream, target, ".", []string{"vendor"}, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(target, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git on the host: %v, want it not written", err)
	}
}
