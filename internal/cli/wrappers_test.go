package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What the copy and varnish handlers ask of the engine — not what the engine does with it, which
// is dockergento's own tests. projectOr calls Resolve with the working directory, not the
// project's root, so every case below carries both: cwd for the calls projectOr itself makes,
// and a distinct root for the calls the handler makes once it has a resolved project.
//

func TestWhatCopyingIntoTheContainerAsks(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	cases := []struct {
		name string
		args []string
		all  bool
	}{
		{"a named path", []string{"app/code"}, false},
		{"--all first", []string{"--all"}, true},
		{"--all not first", []string{"app/code", "--all"}, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}

			code := copyInto(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("copy-to-container %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "CopyInto", Dir: cwd, Paths: tt.args, All: tt.all},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWhatCopyingOutOfTheContainerAsks(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	args := []string{"generated", "var/log"}

	code := copyFrom(args, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("copy-from-container %v = %d, want exitOK", args, code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "CopyFrom", Dir: cwd, Paths: args},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// Neither copy command has anything to ask about with no path — not even whether there is a
// project to ask it of.
func TestCopyingWithNoPathIsRefused(t *testing.T) {
	cases := []struct {
		name string
		run  func(stdout, stderr io.Writer, jsonOutput bool) int
	}{
		{"copy-to-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyInto(nil, stdout, stderr, jsonOutput)
		}},
		{"copy-from-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyFrom(nil, stdout, stderr, jsonOutput)
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			code := tt.run(io.Discard, io.Discard, false)

			if code != exitUsage {
				t.Fatalf("%s with no path = %d, want exitUsage", tt.name, code)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("%s with no path asked the engine %v, want nothing asked at all", tt.name, fake.calls)
			}
		})
	}
}

func TestACopyTheEngineRefusesIsReported(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	refused := errors.New("docker: no")

	cases := []struct {
		name string
		run  func(stdout, stderr io.Writer, jsonOutput bool) int
		want []call
	}{
		{"copy-to-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyInto([]string{"app/code"}, stdout, stderr, jsonOutput)
		}, []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "CopyInto", Dir: cwd, Paths: []string{"app/code"}, All: false},
		}},
		{"copy-from-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyFrom([]string{"generated"}, stdout, stderr, jsonOutput)
		}, []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "CopyFrom", Dir: cwd, Paths: []string{"generated"}},
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			fake.outcomes = []outcome{{}, {err: refused}}

			code := tt.run(io.Discard, io.Discard, false)

			if code != exitDocker {
				t.Fatalf("%s refused by the engine = %d, want exitDocker", tt.name, code)
			}

			// A trivial fake substitution and a real Docker refusal would land on the same exit
			// code by coincidence, so the log is what proves the fake was actually asked.
			if diff := cmp.Diff(tt.want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// The two edits are inverses, and this is the only place in Go that says so: the same marker,
// commented in for one direction and out for the other.
func TestWhatVarnishAsks(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"

	t.Run("on", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}

		code := varnish(true, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("varnish-on = %d, want exitOK", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: varnishService,
				Command: []string{"sed", "-i",
					`s/^[^#]\+return(pass); #skip-varnish/#return(pass); #skip-varnish/g`,
					"/etc/varnish/default.vcl"},
				Options: terminalOptions("root")},
			{Method: "Restart", Dir: root, Services: []string{varnishService}},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"bin/magento", "cache:enable", "full_page"},
				Options: terminalOptions("")},
		}

		if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
			t.Fatalf("varnish-on asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("off", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}

		code := varnish(false, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("varnish-off = %d, want exitOK", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: varnishService,
				Command: []string{"sed", "-i",
					`s/#\+return(pass); #skip-varnish/ return(pass); #skip-varnish/g`,
					"/etc/varnish/default.vcl"},
				Options: terminalOptions("root")},
			{Method: "Restart", Dir: root, Services: []string{varnishService}},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"bin/magento", "cache:disable", "full_page"},
				Options: terminalOptions("")},
			// Turning it off leaves pages cached by it behind: purge and cache:clean run next,
			// each through its own projectOr, which is why Resolve is asked twice more.
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"sh", "-c", "rm -rf " + strings.Join(generated, " ")},
				Options: terminalOptions("")},
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"php", "./bin/magento", "cache:clean"},
				Options: terminalOptions("")},
		}

		if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
			t.Fatalf("varnish-off asked of the engine (-want +got):\n%s", diff)
		}
	})
}

func TestAFailedEditStopsBeforeTheRestart(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {err: refused}}

	code := varnish(true, io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("varnish-on with a failed edit = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: root, Service: varnishService,
			Command: []string{"sed", "-i",
				`s/^[^#]\+return(pass); #skip-varnish/#return(pass); #skip-varnish/g`,
				"/etc/varnish/default.vcl"},
			Options: terminalOptions("root")},
	}

	if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
		t.Fatalf("asked of the engine after a failed edit (-want +got):\n%s", diff)
	}
}

func TestOutsideAProjectNothingIsAsked(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	fake := answering(t) // fake.project stays at its zero value: no project

	code := copyInto([]string{"app/code"}, io.Discard, io.Discard, false)

	if code != exitProject {
		t.Fatalf("copy-to-container outside a project = %d, want exitProject", code)
	}

	want := []call{{Method: "Resolve", Dir: cwd}}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestWhatMysqldumpAsks is the one command whose Dir is here() rather than the resolved
// project's root — the same here() vs project.Root inconsistency design.md leaves open for the
// copy commands, not fixed here either.
func TestWhatMysqldumpAsks(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v, want no error", err)
	}

	path := filepath.Join(t.TempDir(), "dump.sql")

	t.Run("success", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		code := dump([]string{path}, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("mysqldump %s = %d, want exitOK", path, code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Dump", Dir: cwd, Path: path},
		}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("--json answers the path it wrote", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		stdout := &strings.Builder{}

		code := dump([]string{path}, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("mysqldump --json %s = %d, want exitOK", path, code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		if envelope.Data["path"] != path {
			t.Fatalf("data.path = %v, want %q", envelope.Data["path"], path)
		}
	})

	// A refused Dump and a real Docker refusal land on the same exit code by coincidence, so the
	// log is what proves the fake was actually asked, the same discipline TestACopyTheEngine
	// RefusesIsReported uses above.
	t.Run("a refused dump is reported", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}
		fake.outcomes = []outcome{{}, {err: errors.New("docker: no")}}

		code := dump([]string{path}, io.Discard, io.Discard, false)

		if code != exitDocker {
			t.Fatalf("mysqldump refused by the engine = %d, want exitDocker", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Dump", Dir: cwd, Path: path},
		}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})
}

// mysqldump has nothing to ask about with no path — not even whether there is a project to ask
// it of, the same as the two copy commands above.
func TestMysqldumpWithNoPathIsRefused(t *testing.T) {
	fake := answering(t)

	code := dump(nil, io.Discard, io.Discard, false)

	if code != exitUsage {
		t.Fatalf("mysqldump with no path = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("mysqldump with no path asked the engine %v, want nothing asked at all", fake.calls)
	}
}
