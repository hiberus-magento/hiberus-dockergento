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

func TestCaptureReturnsWhatTheExecutablePrinted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talker")
	script := "#!/bin/sh\nread line && echo \"read $line\"\necho out\necho err >&2\nexit 3\n"

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("standard output alone, standard error dropped, standard input closed", func(t *testing.T) {
		output, err := (Runner{Executable: path}).Capture([]string{"x"}, false)
		if err != nil {
			t.Fatalf("= %v, want a failing exit code not to be an error", err)
		}

		if string(output) != "out\n" {
			t.Fatalf("output = %q, want only stdout", output)
		}
	})

	t.Run("both, in order, when combined", func(t *testing.T) {
		output, err := (Runner{Executable: path}).Capture(nil, true)
		if err != nil {
			t.Fatal(err)
		}

		if string(output) != "out\nerr\n" {
			t.Fatalf("output = %q, want stdout and stderr together", output)
		}
	})

	t.Run("nothing to run is an error", func(t *testing.T) {
		if _, err := (Runner{Executable: filepath.Join(t.TempDir(), "no")}).Capture(nil, false); err == nil {
			t.Fatal("= nil, want it said")
		}
	})
}

// TestDirIsWhereItRuns: every command of this binary answers about the directory it runs in, and
// a branch environment is a directory, so the MCP server runs its tools there.
func TestDirIsWhereItRuns(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "where")
	elsewhere := filepath.Join(root, "feature")

	if err := os.Mkdir(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("#!/bin/sh\npwd -P\nexit 4\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	want, err := filepath.EvalSymlinks(elsewhere)
	if err != nil {
		t.Fatal(err)
	}

	output, status, err := (Runner{Executable: path, Dir: elsewhere}).CaptureStatus(nil, false)
	if err != nil {
		t.Fatalf("= %v, want no error", err)
	}

	if string(output) != want+"\n" || status != 4 {
		t.Fatalf("output, status = %q, %d, want %q, 4", output, status, want+"\n")
	}

	t.Run("left empty, this process's own", func(t *testing.T) {
		here, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		here, _ = filepath.EvalSymlinks(here)

		output, _, err := (Runner{Executable: path}).CaptureStatus(nil, false)
		if err != nil || string(output) != here+"\n" {
			t.Fatalf("output = %q, %v, want %q", output, err, here+"\n")
		}
	})

	t.Run("one that is not there is an error", func(t *testing.T) {
		if _, _, err := (Runner{Executable: path, Dir: filepath.Join(root, "gone")}).CaptureStatus(nil, false); err == nil {
			t.Fatal("= nil, want it said")
		}
	})
}
