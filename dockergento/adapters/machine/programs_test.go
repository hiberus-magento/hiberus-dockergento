package machine

import (
	"os"
	"path/filepath"
	"testing"
)

// Quiet sends a program's output to the null device even when streams were given, and still
// reports how it exited.
func TestAQuietProgramSaysNothingAndStillFails(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close() //nolint:errcheck

	quiet := Programs{Stdout: out, Stderr: out, Quiet: true}

	if err := quiet.Run(t.TempDir(), "sh", "-c", "echo noise; echo more >&2"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if contents, _ := os.ReadFile(out.Name()); len(contents) != 0 {
		t.Errorf("a quiet program wrote %q", contents)
	}

	if err := quiet.Run(t.TempDir(), "sh", "-c", "exit 3"); err == nil {
		t.Error("want a failing program reported")
	}
}
