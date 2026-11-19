package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What mysql asks of the engine. The handler (mysql.go:18) reads -q, -i, -d and -a, resolves the
// project and asks for one of four things: a query, an import, a dump fed on stdin, or a session.
// What each of them is — the client inside the db container, the DEFINER clauses, the domain kept
// across an import, the development settings after it — is the engine's, and is tested there.
//
// The session is not pinned here: it is what mysql does when stdin is a terminal, and under
// `go test` stdin never is. Every test that reaches the engine therefore takes the other side of
// that fork deliberately, and says so.
//

func shop(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	return fake
}

func TestMysqlQueryRunsTheStatementAndKeepsItsStatus(t *testing.T) {
	fake := shop(t)
	fake.outcomes = []outcome{{}, {status: 17, captured: "1\n"}}
	cwd := here()

	var stdout bytes.Buffer
	if code := mysql([]string{"-q", "SELECT 1"}, &stdout, io.Discard, false); code != 17 {
		t.Fatalf("mysql -q = %d, want the client's own 17", code)
	}

	if stdout.String() != "1\n" {
		t.Errorf("stdout = %q, want the statement's own output", stdout.String())
	}

	want := []call{{Method: "Resolve", Dir: cwd}, {Method: "Query", Dir: cwd, Statement: "SELECT 1"}}
	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestMysqlImportPassesItsOptions(t *testing.T) {
	dump := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.WriteFile(dump, []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		args    []string
		options app.ImportOptions
	}{
		{"a plain import", []string{"-i", dump}, app.ImportOptions{File: dump}},
		{"-d strips the DEFINER clauses", []string{"-d", "-i", dump}, app.ImportOptions{File: dump, CleanDefiners: true}},
		{"-a anonymises afterwards", []string{"-i", dump, "-a"}, app.ImportOptions{File: dump, Anonymise: true}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := shop(t)
			cwd := here()

			if code := mysql(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("mysql %v = %d, want exitOK", tt.args, code)
			}

			want := []call{{Method: "Resolve", Dir: cwd}, {Method: "Import", Dir: cwd, ImportOptions: tt.options}}
			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMysqlImportExpandsTheTilde: the path arrives as a value, so the shell never expanded it.
func TestMysqlImportExpandsTheTilde(t *testing.T) {
	fake := shop(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, "dump.sql"), []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if code := mysql([]string{"-i", "~/dump.sql"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("mysql -i ~/dump.sql = %d, want exitOK", code)
	}

	if got := fake.calls[len(fake.calls)-1].ImportOptions.File; got != filepath.Join(home, "dump.sql") {
		t.Errorf("imported %q, want the file in HOME", got)
	}
}

// TestMysqlImportOfAMissingFileImportsNothing: a mistyped path is said and nothing is imported.
func TestMysqlImportOfAMissingFileImportsNothing(t *testing.T) {
	fake := shop(t)

	var stderr bytes.Buffer
	if code := mysql([]string{"-i", "/nowhere/dump.sql"}, io.Discard, &stderr, false); code != exitOK {
		t.Fatalf("mysql -i on a missing file = %d, want exitOK", code)
	}

	if !strings.Contains(stderr.String(), "No such file: /nowhere/dump.sql") {
		t.Errorf("stderr = %q, want it to name the missing file", stderr.String())
	}

	for _, c := range fake.calls {
		if c.Method == "Import" {
			t.Fatalf("imported a file that does not exist: %v", fake.calls)
		}
	}
}

// TestMysqlFeedsADumpOnStdin: no statement, no file, and a stdin that is not a terminal — which
// under go test it never is — is a dump to feed, with the client's status kept.
func TestMysqlFeedsADumpOnStdin(t *testing.T) {
	fake := shop(t)
	fake.outcomes = []outcome{{}, {status: 17}}
	cwd := here()

	if code := mysql(nil, io.Discard, io.Discard, false); code != 17 {
		t.Fatalf("mysql < dump = %d, want the client's own 17", code)
	}

	want := []call{{Method: "Resolve", Dir: cwd}, {Method: "Feed", Dir: cwd}}
	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestMysqlRefusesWhatItDoesNotKnow: a malformed invocation is a usage error before anything is
// resolved.
func TestMysqlRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, args := range [][]string{{"--nonsense"}, {"-q"}, {"-i"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fake := shop(t)

			if code := mysql(args, io.Discard, io.Discard, true); code != exitUsage {
				t.Fatalf("mysql %v = %d, want exitUsage", args, code)
			}

			if len(fake.calls) != 0 {
				t.Errorf("asked the engine %v, want nothing", fake.calls)
			}
		})
	}
}
