package dockerd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// tree makes a small vendor: two files in two directories, 11 bytes in all.
func tree(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "vendor")

	for name, content := range map[string]string{"a/one.php": "hello", "b/two.php": "world!"} {
		path := filepath.Join(root, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// counted keeps the last thing a meter was told, and how many times it was told anything.
type counted struct{ bytes, files, calls int64 }

func (c *counted) meter(bytes, files int64) { c.bytes, c.files, c.calls = bytes, files, c.calls+1 }

func TestPackingCountsTheBytesAndFilesItCarriesWithoutChangingThem(t *testing.T) {
	source := tree(t)

	var plain, metered bytes.Buffer

	if err := pack(&plain, source, nil); err != nil {
		t.Fatal(err)
	}

	seen := &counted{}
	if err := pack(&metered, source, seen.meter); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(plain.Bytes(), metered.Bytes()) {
		t.Fatal("the metered stream differs from the plain one")
	}

	if seen.bytes != 11 || seen.files != 2 {
		t.Fatalf("meter told %d bytes and %d files, want 11 and 2", seen.bytes, seen.files)
	}
}

func TestUnpackingCountsTheBytesAndFilesItWritesWithoutChangingThem(t *testing.T) {
	var stream bytes.Buffer

	if err := pack(&stream, tree(t), nil); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "vendor")
	seen := &counted{}

	if err := unpack(&stream, target, "vendor", seen.meter); err != nil {
		t.Fatal(err)
	}

	if seen.bytes != 11 || seen.files != 2 {
		t.Fatalf("meter told %d bytes and %d files, want 11 and 2", seen.bytes, seen.files)
	}

	for name, want := range map[string]string{"a/one.php": "hello", "b/two.php": "world!"} {
		got, err := os.ReadFile(filepath.Join(target, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
}

// Without a meter, nothing is counted and nothing is called: the copy is the copy it always was.
func TestUnpackingWithoutAMeterWritesTheSameFiles(t *testing.T) {
	var stream bytes.Buffer

	if err := pack(&stream, tree(t), nil); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "vendor")

	if err := unpack(&stream, target, "vendor", nil); err != nil {
		t.Fatal(err)
	}

	if got, err := os.ReadFile(filepath.Join(target, "b", "two.php")); err != nil || string(got) != "world!" {
		t.Fatalf("b/two.php = %q, %v", got, err)
	}
}
