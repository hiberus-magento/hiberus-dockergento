package cli

import (
	"encoding/json"
	"errors"
	"io"
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
			cwd := here()

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
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

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
	refused := errors.New("docker: no")

	cases := []struct {
		name string
		run  func(stdout, stderr io.Writer, jsonOutput bool) int
		want func(cwd string) []call
	}{
		{"copy-to-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyInto([]string{"app/code"}, stdout, stderr, jsonOutput)
		}, func(cwd string) []call {
			return []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "CopyInto", Dir: cwd, Paths: []string{"app/code"}, All: false},
			}
		}},
		{"copy-from-container", func(stdout, stderr io.Writer, jsonOutput bool) int {
			return copyFrom([]string{"generated"}, stdout, stderr, jsonOutput)
		}, func(cwd string) []call {
			return []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "CopyFrom", Dir: cwd, Paths: []string{"generated"}},
			}
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			fake.outcomes = []outcome{{}, {err: refused}}
			cwd := here()

			code := tt.run(io.Discard, io.Discard, false)

			if code != exitDocker {
				t.Fatalf("%s refused by the engine = %d, want exitDocker", tt.name, code)
			}

			// A trivial fake substitution and a real Docker refusal would land on the same exit
			// code by coincidence, so the log is what proves the fake was actually asked.
			if diff := cmp.Diff(tt.want(cwd), fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// The two edits are inverses, and this is the only place in Go that says so: the same marker,
// commented in for one direction and out for the other.
func TestWhatVarnishAsks(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"

	t.Run("on", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		cwd := here()

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
		cwd := here()

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
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {err: refused}}
	cwd := here()

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

// TestAFailedVCLEditReportsTheContainersCode is the sibling branch TestAFailedEditStopsBefore
// TheRestart never covered: the sed exits non-zero with no Go-level error, which report() maps
// to exitOK today because it only looks at err. Proved once against varnish-on — the guard code
// is shared with varnish-off.
func TestAFailedVCLEditReportsTheContainersCode(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"

	t.Run("text", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {status: 42}}
		cwd := here()
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := varnish(true, stdout, stderr, false)

		if code != 42 {
			t.Fatalf("varnish-on with a failed edit = %d, want the container's own 42", code)
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

		if strings.Contains(stdout.String(), "Varnish cache enabled!") {
			t.Fatalf("stdout = %q, want no success line on a failed edit", stdout.String())
		}

		if !strings.Contains(stderr.String(), "VCL edit") {
			t.Fatalf("stderr = %q, want a line naming the VCL edit as the failing step", stderr.String())
		}
	})

	t.Run("--json", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {status: 42}}
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := varnish(true, stdout, stderr, true)

		if code != 42 {
			t.Fatalf("varnish-on --json with a failed edit = %d, want 42", code)
		}

		if stdout.String() != "" {
			t.Fatalf("stdout = %q, want no success document on a failed edit", stdout.String())
		}

		var envelope struct {
			OK    bool `json:"ok"`
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
		}

		if envelope.OK {
			t.Fatalf("envelope.ok = true, want false")
		}

		if envelope.Error.Code != 42 {
			t.Fatalf("envelope.error.code = %d, want 42", envelope.Error.Code)
		}
	})
}

// TestAFailedCacheToggleStopsBeforeThePurge proves the same missing branch for the second guard,
// against varnish-off specifically: it is the one command with a cleanup step (purge, then
// cache:clean) that must never run once the toggle itself is reported as failed.
func TestAFailedCacheToggleStopsBeforeThePurge(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"

	t.Run("text", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {}, {}, {status: 42}}
		cwd := here()
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := varnish(false, stdout, stderr, false)

		if code != 42 {
			t.Fatalf("varnish-off with a failed cache toggle = %d, want the container's own 42", code)
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
		}

		if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
			t.Fatalf("asked of the engine after a failed cache toggle (-want +got):\n%s", diff)
		}

		if strings.Contains(stdout.String(), "Varnish cache disabled!") {
			t.Fatalf("stdout = %q, want no success line on a failed cache toggle", stdout.String())
		}

		if !strings.Contains(stderr.String(), "cache:disable") {
			t.Fatalf("stderr = %q, want a line naming the cache-toggle step as the failing one", stderr.String())
		}
	})

	t.Run("--json", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {}, {}, {status: 42}}
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := varnish(false, stdout, stderr, true)

		if code != 42 {
			t.Fatalf("varnish-off --json with a failed cache toggle = %d, want 42", code)
		}

		if stdout.String() != "" {
			t.Fatalf("stdout = %q, want no success document on a failed cache toggle", stdout.String())
		}

		var envelope struct {
			OK    bool `json:"ok"`
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
		}

		if envelope.OK {
			t.Fatalf("envelope.ok = true, want false")
		}

		if envelope.Error.Code != 42 {
			t.Fatalf("envelope.error.code = %d, want 42", envelope.Error.Code)
		}
	})
}

// TestWhatDebugOffAsks proves the whole call log is exactly three entries — Resolve, Exec, then
// Compose — with no fourth entry and in particular no Start call: the shell script's own
// defensive start_service_if_not_running.sh probe is dropped on purpose, not replicated.
func TestWhatDebugOffAsks(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	root := "/code/shop"

	t.Run("text", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		cwd := here()

		code := debugOff(io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("debug-off = %d, want exitOK", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
					"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
				Options: terminalOptions("")},
			{Method: "Compose", Dir: root, Command: []string{"restart", "phpfpm"}},
		}

		if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}

		// Separately from the diff above: the reason this scenario exists in the first place is
		// that the shell script's own defensive start is dropped, not replicated. A cmp.Diff that
		// fails on an unexpected fourth entry reads as an ordinary mismatch; this says plainly
		// what a fourth call would mean.
		if len(fake.calls) != 3 {
			t.Fatalf("debug-off asked the engine %d things, want exactly 3 (no Start call)", len(fake.calls))
		}
	})

	t.Run("json", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}

		stdout := &strings.Builder{}

		code := debugOff(stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("debug-off --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data struct {
				Xdebug bool `json:"xdebug"`
			} `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		if envelope.Data.Xdebug {
			t.Fatalf("data.xdebug = true, want false")
		}
	})
}

// TestAFailedDebugOffEditReportsTheContainersStatus mirrors TestAFailedVCLEditReportsTheContainers
// Code exactly: the sed exits non-zero with no Go-level error, and this is also the seam-visible
// half of the disclosed behaviour change — with the environment stopped, this is what answers
// non-zero instead of the shell script silently starting everything.
func TestAFailedDebugOffEditReportsTheContainersStatus(t *testing.T) {
	root := "/code/shop"

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {status: 42}}
	cwd := here()
	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	code := debugOff(stdout, stderr, false)

	if code != 42 {
		t.Fatalf("debug-off with a failed edit = %d, want the container's own 42", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine after a failed edit (-want +got):\n%s", diff)
	}

	if strings.Contains(stdout.String(), "xdebug disabled") {
		t.Fatalf("stdout = %q, want no success line on a failed edit", stdout.String())
	}

	if !strings.Contains(stderr.String(), "ini edit") {
		t.Fatalf("stderr = %q, want a line naming the ini edit as the failing step", stderr.String())
	}
}

// TestADebugOffEditErrorStopsBeforeTheRestart mirrors TestAFailedEditStopsBeforeTheRestart exactly:
// the err != nil branch, distinct from the non-zero-status one above.
func TestADebugOffEditErrorStopsBeforeTheRestart(t *testing.T) {
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {err: refused}}
	cwd := here()

	code := debugOff(io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("debug-off with an edit error = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine after an edit error (-want +got):\n%s", diff)
	}
}

// TestAFailedDebugOffRestartReturnsTheComposeStatus proves the restart's own non-zero status is
// returned as-is, unwrapped — the one place debug-off's failure handling diverges from the sed
// edit's: no failure() call names a step, and no structured ok: false object is written under
// --json, mirroring dockerCompose()'s own handling of the identical (int, error) pair.
func TestAFailedDebugOffRestartReturnsTheComposeStatus(t *testing.T) {
	root := "/code/shop"

	t.Run("text", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {}, {status: 42}}
		cwd := here()
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := debugOff(stdout, stderr, false)

		if code != 42 {
			t.Fatalf("debug-off with a failed restart = %d, want the container's own 42", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
					"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
				Options: terminalOptions("")},
			{Method: "Compose", Dir: root, Command: []string{"restart", "phpfpm"}},
		}

		if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
			t.Fatalf("asked of the engine after a failed restart (-want +got):\n%s", diff)
		}

		if stdout.String() != "" {
			t.Fatalf("stdout = %q, want nothing at all — no success line and no failure() wrapping", stdout.String())
		}

		if stderr.String() != "" {
			t.Fatalf("stderr = %q, want nothing at all — the raw status is the only signal", stderr.String())
		}
	})

	t.Run("--json", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = []outcome{{}, {}, {status: 42}}
		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := debugOff(stdout, stderr, true)

		if code != 42 {
			t.Fatalf("debug-off --json with a failed restart = %d, want 42", code)
		}

		if stdout.String() != "" {
			t.Fatalf("stdout = %q, want no success document on a failed restart", stdout.String())
		}

		if stderr.String() != "" {
			t.Fatalf("stderr = %q, want no structured ok:false object either — the raw status is "+
				"the only signal", stderr.String())
		}
	})
}

// TestADebugOffRestartErrorIsReported proves the other half of the restart's (int, error) pair:
// a Go-level error is reported through the same fault-mapping report() uses elsewhere, the same
// path dockerCompose()'s own err != nil branch takes.
func TestADebugOffRestartErrorIsReported(t *testing.T) {
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {}, {err: refused}}
	cwd := here()

	code := debugOff(io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("debug-off with a restart error = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
		{Method: "Compose", Dir: root, Command: []string{"restart", "phpfpm"}},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine after a restart error (-want +got):\n%s", diff)
	}
}

func TestOutsideAProjectNothingIsAsked(t *testing.T) {
	fake := answering(t) // fake.project stays at its zero value: no project
	cwd := here()

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
	path := filepath.Join(t.TempDir(), "dump.sql")

	t.Run("success", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}
		cwd := here()

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
		cwd := here()

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

//
// The two pure parsers debug-on's readback will use: no engine, no caller yet. Fixture-testable
// on any runner, no t.Skip.
//

const phpVWithXdebugV2 = `PHP 7.4.33 (cli) (built: Nov 14 2022 18:31:42) ( NTS )
Copyright (c) The PHP Group
Zend Engine v3.4.0, Copyright (c) Zend Technologies
    with Zend OPcache v7.4.33, Copyright (c), by Zend Technologies
    with Xdebug v2.9.8, Copyright (c) 2020, by Derick Rethans
`

const phpVWithXdebugV3 = `PHP 8.1.27 (cli) (built: Nov 19 2023 00:00:00) (NTS)
Copyright (c) The PHP Group
Zend Engine v4.1.27, Copyright (c) Zend Technologies
    with Zend OPcache v8.1.27, Copyright (c), by Zend Technologies
    with Xdebug v3.1.5, Copyright (c) 2002-2021, by Derick Rethans
`

// phpVWithNoXdebugAtAll carries no line containing "Xdebug v" at all — not one where such a line
// merely sits late — the case the shell's own `set -euo pipefail` aborted on before ever reaching
// its own else clause (debug-on.sh:27, :36).
const phpVWithNoXdebugAtAll = `PHP 8.1.27 (cli) (built: Nov 19 2023 00:00:00) (NTS)
Copyright (c) The PHP Group
Zend Engine v4.1.27, Copyright (c) Zend Technologies
    with Zend OPcache v8.1.27, Copyright (c), by Zend Technologies
`

// phpVWithUnrecognizedXdebugVersion matches "Xdebug v" but neither "v2." nor "v3." — the only
// case the shell's own else branch (debug-on.sh:35-36) could ever actually reach.
const phpVWithUnrecognizedXdebugVersion = `PHP 8.3.0 (cli) (built: Nov 19 2024 00:00:00) (NTS)
Copyright (c) The PHP Group
Zend Engine v4.3.0, Copyright (c) Zend Technologies
    with Zend OPcache v8.3.0, Copyright (c), by Zend Technologies
    with Xdebug v4.0.0, Copyright (c) 2002-2024, by Derick Rethans
`

func TestXdebugVersionClassifiesPhpVOutput(t *testing.T) {
	cases := []struct {
		name       string
		output     string
		wantFamily string
	}{
		{"a v2 version line", phpVWithXdebugV2, "v2"},
		{"a v3 version line", phpVWithXdebugV3, "v3"},
		{"no Xdebug line at all", phpVWithNoXdebugAtAll, ""},
		{"Xdebug v4, matching neither v2 nor v3", phpVWithUnrecognizedXdebugVersion, ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, family := xdebugVersion(tt.output)

			if family != tt.wantFamily {
				t.Fatalf("xdebugVersion(%q) family = %q, want %q", tt.output, family, tt.wantFamily)
			}
		})
	}
}

func TestXdebugVersionExtractsTheLineVerbatim(t *testing.T) {
	version, _ := xdebugVersion(phpVWithXdebugV3)

	want := "    with Xdebug v3.1.5, Copyright (c) 2002-2021, by Derick Rethans"
	if version != want {
		t.Fatalf("xdebugVersion(...) version = %q, want %q", version, want)
	}

	if version, _ := xdebugVersion(phpVWithNoXdebugAtAll); version != "" {
		t.Fatalf("xdebugVersion(no Xdebug line) version = %q, want \"\"", version)
	}
}

// phpIWithBothVersionFamilies is the discriminating fixture: it carries both
// xdebug.remote_host/xdebug.remote_port (v2) and xdebug.client_host/xdebug.client_port (v3),
// alongside the idekey they share and an unrelated key neither selection should ever return. A
// v3 selection that wrongly also accepted a v2 key cannot pass here by the v2 key merely being
// absent — and the same holds in the opposite direction.
const phpIWithBothVersionFamilies = `xdebug.mode => debug,develop => debug,develop
xdebug.client_host => no value => no value
xdebug.client_port => 9003 => 9003
xdebug.idekey => VSCODE => VSCODE
xdebug.remote_host => no value => no value
xdebug.remote_port => 9000 => 9000
xdebug.start_with_request => yes => yes
`

func TestXdebugSettingsSelectsOnlyTheGivenFamilysKeys(t *testing.T) {
	t.Run("v2 selects only the remote_* pair and idekey", func(t *testing.T) {
		got := xdebugSettings(phpIWithBothVersionFamilies, "v2")

		keys := xdebugSettingKeysOf(got)
		want := []string{"xdebug.idekey", "xdebug.remote_host", "xdebug.remote_port"}

		if diff := cmp.Diff(want, keys); diff != "" {
			t.Fatalf("xdebugSettings(..., \"v2\") keys (-want +got):\n%s", diff)
		}

		for _, setting := range got {
			if setting.Key == "xdebug.client_host" || setting.Key == "xdebug.client_port" {
				t.Fatalf("xdebugSettings(..., \"v2\") wrongly selected the v3 key %q", setting.Key)
			}
		}
	})

	t.Run("v3 selects only the client_* pair and idekey, on the same fixture", func(t *testing.T) {
		got := xdebugSettings(phpIWithBothVersionFamilies, "v3")

		// Selection order follows php -i's own line order, the same as the shell's
		// `grep -e ... -e ... -e ...` does — not the order the wanted keys are listed in. On
		// this fixture that means client_host and client_port (which sit earlier in the
		// output) before idekey.
		keys := xdebugSettingKeysOf(got)
		want := []string{"xdebug.client_host", "xdebug.client_port", "xdebug.idekey"}

		if diff := cmp.Diff(want, keys); diff != "" {
			t.Fatalf("xdebugSettings(..., \"v3\") keys (-want +got):\n%s", diff)
		}

		for _, setting := range got {
			if setting.Key == "xdebug.remote_host" || setting.Key == "xdebug.remote_port" {
				t.Fatalf("xdebugSettings(..., \"v3\") wrongly selected the v2 key %q", setting.Key)
			}
		}
	})

	t.Run("neither family selects nothing", func(t *testing.T) {
		if got := xdebugSettings(phpIWithBothVersionFamilies, ""); len(got) != 0 {
			t.Fatalf("xdebugSettings(..., \"\") = %v, want nothing selected", got)
		}
	})
}

func TestXdebugSettingsTrimsToKeyAndValue(t *testing.T) {
	got := xdebugSettings(phpIWithBothVersionFamilies, "v2")

	for _, setting := range got {
		if setting.Key == "xdebug.remote_port" && setting.Value != "9000" {
			t.Fatalf("xdebugSettings(...) xdebug.remote_port value = %q, want %q", setting.Value, "9000")
		}
	}
}

// xdebugSettingKeysOf is a small test helper: the key half of each selected setting, in
// selection order, which is what the discriminating assertions above compare.
func xdebugSettingKeysOf(settings []xdebugSetting) []string {
	keys := make([]string, len(settings))
	for i, setting := range settings {
		keys[i] = setting.Key
	}

	return keys
}
