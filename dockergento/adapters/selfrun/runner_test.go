package selfrun

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// recorder writes a script to t.TempDir() that logs the arguments it was given, one per line, and
// exits with the given code.
func recorder(t *testing.T, exit int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "recorder")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\nexit %d\n", path+".args", exit)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestItRunsTheExecutableItWasGiven(t *testing.T) {
	path := recorder(t, 7)

	code, err := (Runner{Executable: path}).Run([]string{"magento", "cache:flush", "--json"})
	if err != nil {
		t.Fatalf("= %v, want no error", err)
	}

	if code != 7 {
		t.Fatalf("exit code = %d, want the executable's own", code)
	}

	arguments, err := os.ReadFile(path + ".args")
	if err != nil {
		t.Fatal(err)
	}

	if string(arguments) != "magento\ncache:flush\n--json\n" {
		t.Fatalf("arguments = %q, want them untouched", arguments)
	}
}

func TestAnExecutableThatIsNotThereIsAnError(t *testing.T) {
	t.Run("nothing at that path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "no-existe")

		if _, err := (Runner{Executable: path}).Run([]string{"install"}); err == nil {
			t.Fatal("a missing executable = nil, want it said")
		}
	})

	t.Run("there, but not executable", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "no-x")
		marker := filepath.Join(root, "ran")

		script := fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker)
		if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := (Runner{Executable: path}).Run([]string{"install"}); err == nil {
			t.Fatal("a non-executable file = nil, want it said")
		}

		if _, err := os.Stat(marker); err == nil {
			t.Fatal("the marker exists, want no exec attempt at all")
		}
	})
}
