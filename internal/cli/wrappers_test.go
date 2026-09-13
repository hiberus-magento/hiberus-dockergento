package cli

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"
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

//
// debug-on, Slice 3a: the failure and refusal paths, everything up to and including Restart.
// None of these need the readback (Slice 3b) to be meaningful — the ones that reach Restart
// stop the fake right there, with an outcome that never lets a Capture call get recorded.
//

// macCopyCalls is what debug-on's macOS-only branch adds to the call log between the uncomment
// edit and the restart: a Resolve and a CopyFrom, both through copyFrom's own here()
// (TestWhatCopyingOutOfTheContainerAsks is the precedent for that shape). debug-on branches on
// the real runtime.GOOS (php.go:100's own check, no override), so a test whose call log spans
// that branch has to account for whatever platform the test binary itself runs on — a genuine
// Linux scenario, written and run on a macOS development machine, would otherwise see calls a
// real Linux CI run never will.
func macCopyCalls(cwd string) []call {
	if runtime.GOOS != "darwin" {
		return nil
	}

	return []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "CopyFrom", Dir: cwd, Paths: []string{"generated"}},
	}
}

// macCopyOutcomes pads a test's outcomes by the same amount macCopyCalls adds to the log, so an
// outcome meant for the call after the macOS branch (Restart, in every Slice 3a test that needs
// one) lands at the right index regardless of platform.
func macCopyOutcomes() []outcome {
	if runtime.GOOS != "darwin" {
		return nil
	}

	return []outcome{{}, {}}
}

// TestDebugOnReportsAnUnresolvableHost is debug-on-host-detection's Decision 2 (renumbered
// Decision 4 in this proposal), made observable for the first time: a docker0 that exists but
// could not be resolved is a failure, not a silent skip.
func TestDebugOnReportsAnUnresolvableHost(t *testing.T) {
	root := "/code/shop"
	unresolved := errors.New("no address found for the docker0 bridge")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {err: unresolved}}
	cwd := here()
	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	code := debugOn(stdout, stderr, true)

	if code != exitDocker {
		t.Fatalf("debug-on with an unresolvable host = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine after an unresolvable host (-want +got):\n%s", diff)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
	}

	if envelope.Error.Type != "docker_failed" {
		t.Fatalf("error.type = %q, want docker_failed — XdebugHost's error is not a core.Refusal", envelope.Error.Type)
	}
}

// TestAFailedDebugOnEditReportsTheContainersStatus mirrors TestAFailedDebugOffEditReportsThe
// ContainersStatus and TestAFailedVCLEditReportsTheContainersCode: whichever of debug-on's three
// edits fails first — a host patch, or the uncomment that always runs afterward — stops the call
// log there, with no further call recorded. Shared verbatim with magento-commands' scenario of
// the same fact, proved once.
func TestAFailedDebugOnEditReportsTheContainersStatus(t *testing.T) {
	root := "/code/shop"

	cases := []struct {
		name     string
		outcomes []outcome
		wantLen  int
	}{
		{"the first host patch", []outcome{{}, {captured: "172.17.0.1"}, {status: 42}}, 3},
		{"the uncomment, after both host patches succeeded",
			[]outcome{{}, {captured: "172.17.0.1"}, {}, {}, {status: 42}}, 5},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}
			fake.outcomes = tt.outcomes
			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := debugOn(stdout, stderr, false)

			if code != 42 {
				t.Fatalf("debug-on with a failed %s = %d, want the container's own 42", tt.name, code)
			}

			if len(fake.calls) != tt.wantLen {
				t.Fatalf("debug-on asked the engine %d things after a failed %s, want exactly %d — "+
					"no CopyFrom, no Restart, no Capture", len(fake.calls), tt.name, tt.wantLen)
			}

			if strings.Contains(stdout.String(), "xdebug configuration") {
				t.Fatalf("stdout = %q, want no banner on a failed edit", stdout.String())
			}

			if !strings.Contains(stderr.String(), "edit") {
				t.Fatalf("stderr = %q, want a line naming the edit as the failing step", stderr.String())
			}
		})
	}
}

// TestADebugOnEditErrorStopsBeforeTheRestart mirrors TestADebugOffEditErrorStopsBeforeTheRestart:
// the err != nil branch of the same guard, distinct from the non-zero-status one above.
func TestADebugOnEditErrorStopsBeforeTheRestart(t *testing.T) {
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	fake.outcomes = []outcome{{}, {captured: "172.17.0.1"}, {err: refused}}
	cwd := here()

	code := debugOn(io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("debug-on with an edit error = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", `s/xdebug\.remote_host=.*/xdebug\.remote_host=172.17.0.1/g`,
				"/usr/local/etc/php/conf.d/xdebug.ini"},
			Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine after an edit error (-want +got):\n%s", diff)
	}
}

// TestDebugOnRefusesFromAnUnregisteredWorktree is new: hm debug-on has never refused from a
// worktree before this port, because debug-on's own restart.sh never re-entered bin/run's own
// gate (proposal Decision 5). Restart's own error is what carries the refusal here.
func TestDebugOnRefusesFromAnUnregisteredWorktree(t *testing.T) {
	root := "/code/shop"
	blocked := core.Refusal{Kind: "blocked_in_worktree", Code: 6,
		Message: "This worktree is not registered", Hint: ""}

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	cwd := here()

	// host is left empty on purpose here: this test is about the restart, not the host patches,
	// and an empty host keeps the outcome slice's own shape simple.
	outcomes := []outcome{{}, {}, {}}
	outcomes = append(outcomes, macCopyOutcomes()...)
	outcomes = append(outcomes, outcome{err: blocked})
	fake.outcomes = outcomes

	code := debugOn(io.Discard, io.Discard, false)

	if code != 6 {
		t.Fatalf("debug-on from an unregistered worktree = %d, want 6", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
	}
	want = append(want, macCopyCalls(cwd)...)
	want = append(want, call{Method: "Restart", Dir: root, Services: []string{phpService}})

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine from an unregistered worktree (-want +got):\n%s", diff)
	}
}

// TestADebugOnRestartErrorIsReported proves the other half of Restart's error, the one that is
// not a core.Refusal: mapped through the same report() every other engine error goes through.
func TestADebugOnRestartErrorIsReported(t *testing.T) {
	root := "/code/shop"
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	cwd := here()

	outcomes := []outcome{{}, {}, {}}
	outcomes = append(outcomes, macCopyOutcomes()...)
	outcomes = append(outcomes, outcome{err: refused})
	fake.outcomes = outcomes

	code := debugOn(io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("debug-on with a restart error = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
	}
	want = append(want, macCopyCalls(cwd)...)
	want = append(want, call{Method: "Restart", Dir: root, Services: []string{phpService}})

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine after a restart error (-want +got):\n%s", diff)
	}
}

// TestDebugOnIgnoresXdebugHostEnvironmentVariable is Decision 8, disclosed: the shell's own
// undocumented escape hatch is gone, and XdebugHost()'s answer is the only thing that decides
// the patched address, even when the calling process's own environment carries XDEBUG_HOST. This
// stops the run at a deliberately failing uncomment edit — the assertion needs the two host
// patches, not the restart or the readback (proposal Decision 8; asserted "without needing the
// restart or the readback to complete").
func TestDebugOnIgnoresXdebugHostEnvironmentVariable(t *testing.T) {
	root := "/code/shop"
	t.Setenv("XDEBUG_HOST", "9.9.9.9")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	cwd := here()
	fake.outcomes = []outcome{
		{}, {captured: "172.17.0.1"}, {}, {}, {err: errors.New("stop here, deliberately")},
	}

	debugOn(io.Discard, io.Discard, false)

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", `s/xdebug\.remote_host=.*/xdebug\.remote_host=172.17.0.1/g`,
				"/usr/local/etc/php/conf.d/xdebug.ini"},
			Options: terminalOptions("")},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", `s/xdebug\.client_host=.*/xdebug\.client_host=172.17.0.1/g`,
				"/usr/local/etc/php/conf.d/xdebug.ini"},
			Options: terminalOptions("")},
		// The deliberately-failing outcome above lands on this call, which is why it is
		// recorded (attempted) but nothing after it is: this is what stops the run before the
		// restart, without needing the restart itself to do anything.
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine with XDEBUG_HOST set in the environment (-want +got):\n%s", diff)
	}

	for _, recorded := range fake.calls {
		if strings.Contains(strings.Join(recorded.Command, " "), "9.9.9.9") {
			t.Fatalf("call %+v used the environment's XDEBUG_HOST, want only XdebugHost()'s answer", recorded)
		}
	}
}

//
// debug-on, Slice 3b: the readback. Both Capture calls, Slice 1's parsers, the banner, and the
// --json envelope — completing what Slice 3a left at a bare exitOK once the restart succeeds.
//

// debugOnDocument is the shape asserted against debug-on --json's data object. Host is a pointer
// so the test can tell JSON null (Decision 2's not-applicable/unresolved case) apart from the
// empty string a plain Go "" would otherwise marshal to.
type debugOnDocument struct {
	Data struct {
		Xdebug   bool              `json:"xdebug"`
		Version  string            `json:"version"`
		Host     *string           `json:"host"`
		Settings map[string]string `json:"settings"`
	} `json:"data"`
}

// TestWhatDebugOnAsks proves the full ordered call log for a successful Linux run — Resolve,
// XdebugHost, both host patches, the uncomment, Restart (not Compose — the opposite choice from
// debug-off's own), both Capture calls — for both v2 and v3 fixtures, plus the --json envelope
// each produces.
func TestWhatDebugOnAsks(t *testing.T) {
	root := "/code/shop"
	host := "172.17.0.1"

	cases := []struct {
		name       string
		phpV       string
		version    string
		wantKey    string
		wantAbsent string
	}{
		{"v2", phpVWithXdebugV2,
			"    with Xdebug v2.9.8, Copyright (c) 2020, by Derick Rethans",
			"xdebug.remote_host", "xdebug.client_host"},
		{"v3", phpVWithXdebugV3,
			"    with Xdebug v3.1.5, Copyright (c) 2002-2021, by Derick Rethans",
			"xdebug.client_host", "xdebug.remote_host"},
	}

	for _, tt := range cases {
		t.Run(tt.name+"/text", func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}
			cwd := here()

			outcomes := []outcome{{}, {captured: host}, {}, {}, {}}
			outcomes = append(outcomes, macCopyOutcomes()...)
			outcomes = append(outcomes, outcome{}, outcome{captured: tt.phpV}, outcome{captured: phpIWithBothVersionFamilies})
			fake.outcomes = outcomes

			stdout := &strings.Builder{}

			code := debugOn(stdout, io.Discard, false)

			if code != exitOK {
				t.Fatalf("debug-on = %d, want exitOK", code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "XdebugHost"},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", `s/xdebug\.remote_host=.*/xdebug\.remote_host=` + host + `/g`,
						"/usr/local/etc/php/conf.d/xdebug.ini"},
					Options: terminalOptions("")},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", `s/xdebug\.client_host=.*/xdebug\.client_host=` + host + `/g`,
						"/usr/local/etc/php/conf.d/xdebug.ini"},
					Options: terminalOptions("")},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
						"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
					Options: terminalOptions("")},
			}
			want = append(want, macCopyCalls(cwd)...)
			want = append(want,
				call{Method: "Restart", Dir: root, Services: []string{phpService}},
				call{Method: "Capture", Dir: root, Service: phpService, Command: []string{"php", "-v"}},
				call{Method: "Capture", Dir: root, Service: phpService, Command: []string{"php", "-i"}},
			)

			if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}

			printed := stdout.String()

			if !strings.Contains(printed, tt.wantKey) {
				t.Fatalf("stdout = %q, want it to carry %q", printed, tt.wantKey)
			}

			if strings.Contains(printed, tt.wantAbsent) {
				t.Fatalf("stdout = %q, wrongly carries the other family's key %q", printed, tt.wantAbsent)
			}

			if !strings.Contains(printed, "xdebug.idekey") {
				t.Fatalf("stdout = %q, want the shared idekey line too", printed)
			}
		})

		t.Run(tt.name+"/json", func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}

			outcomes := []outcome{{}, {captured: host}, {}, {}, {}}
			outcomes = append(outcomes, macCopyOutcomes()...)
			outcomes = append(outcomes, outcome{}, outcome{captured: tt.phpV}, outcome{captured: phpIWithBothVersionFamilies})
			fake.outcomes = outcomes

			stdout := &strings.Builder{}

			code := debugOn(stdout, io.Discard, true)

			if code != exitOK {
				t.Fatalf("debug-on --json = %d, want exitOK", code)
			}

			var envelope debugOnDocument
			if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
			}

			if !envelope.Data.Xdebug {
				t.Fatalf("data.xdebug = false, want true")
			}

			if envelope.Data.Version != tt.version {
				t.Fatalf("data.version = %q, want the verbatim line %q", envelope.Data.Version, tt.version)
			}

			if envelope.Data.Host == nil || *envelope.Data.Host != host {
				t.Fatalf("data.host = %v, want %q", envelope.Data.Host, host)
			}

			if _, wrong := envelope.Data.Settings[tt.wantAbsent]; wrong {
				t.Fatalf("data.settings wrongly carries the other family's key %q", tt.wantAbsent)
			}

			if _, ok := envelope.Data.Settings[tt.wantKey]; !ok {
				t.Fatalf("data.settings is missing %q: %v", tt.wantKey, envelope.Data.Settings)
			}
		})
	}
}

// TestWhatDebugOnAsksXdebugAbsent covers Decision 1, made observable for the first time: the
// enabling work already completed is recorded exactly as in the installed case, no second
// Capture call is ever recorded, and the command still exits 0 — for both the fixture with no
// Xdebug line at all and the one matching neither v2 nor v3.
func TestWhatDebugOnAsksXdebugAbsent(t *testing.T) {
	root := "/code/shop"
	host := "172.17.0.1"

	cases := []struct {
		name string
		phpV string
	}{
		{"not installed at all", phpVWithNoXdebugAtAll},
		{"an unrecognized version", phpVWithUnrecognizedXdebugVersion},
	}

	for _, tt := range cases {
		t.Run(tt.name+"/text", func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}
			cwd := here()

			outcomes := []outcome{{}, {captured: host}, {}, {}, {}}
			outcomes = append(outcomes, macCopyOutcomes()...)
			outcomes = append(outcomes, outcome{}, outcome{captured: tt.phpV})
			fake.outcomes = outcomes

			stdout := &strings.Builder{}

			code := debugOn(stdout, io.Discard, false)

			if code != exitOK {
				t.Fatalf("debug-on with %s = %d, want exitOK", tt.name, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "XdebugHost"},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", `s/xdebug\.remote_host=.*/xdebug\.remote_host=` + host + `/g`,
						"/usr/local/etc/php/conf.d/xdebug.ini"},
					Options: terminalOptions("")},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", `s/xdebug\.client_host=.*/xdebug\.client_host=` + host + `/g`,
						"/usr/local/etc/php/conf.d/xdebug.ini"},
					Options: terminalOptions("")},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
						"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
					Options: terminalOptions("")},
			}
			want = append(want, macCopyCalls(cwd)...)
			want = append(want,
				call{Method: "Restart", Dir: root, Services: []string{phpService}},
				call{Method: "Capture", Dir: root, Service: phpService, Command: []string{"php", "-v"}},
			)

			if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}

			if !strings.Contains(stdout.String(), "Xdebug is not installed") {
				t.Fatalf("stdout = %q, want it to report Xdebug as not installed", stdout.String())
			}
		})

		t.Run(tt.name+"/json", func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}

			outcomes := []outcome{{}, {captured: host}, {}, {}, {}}
			outcomes = append(outcomes, macCopyOutcomes()...)
			outcomes = append(outcomes, outcome{}, outcome{captured: tt.phpV})
			fake.outcomes = outcomes

			stdout := &strings.Builder{}

			code := debugOn(stdout, io.Discard, true)

			if code != exitOK {
				t.Fatalf("debug-on --json with %s = %d, want exitOK", tt.name, code)
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

			if strings.Contains(stdout.String(), `"version"`) || strings.Contains(stdout.String(), `"settings"`) {
				t.Fatalf("stdout = %q, want no version/host/settings keys at all", stdout.String())
			}
		})
	}
}

// TestWhatDebugOnAsksOnMac proves the macOS success path end to end: no host-patching Exec
// calls, but the uncomment Exec, the warning line, CopyFrom, Restart, and the completed readback
// with host reported as absent. Skipped off macOS the same way TestTheInvocationsThatRewriteThe
// HostsTree is: debug-on branches on the real runtime.GOOS, with no override.
func TestWhatDebugOnAsksOnMac(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS copy branch only exists on macOS")
	}

	root := "/code/shop"

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	cwd := here()
	fake.outcomes = []outcome{
		{}, {captured: ""}, {},
		{}, {},
		{}, {captured: phpVWithXdebugV3}, {captured: phpIWithBothVersionFamilies},
	}

	stdout := &strings.Builder{}

	code := debugOn(stdout, io.Discard, false)

	if code != exitOK {
		t.Fatalf("debug-on on macOS = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "XdebugHost"},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
				"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
			Options: terminalOptions("")},
		{Method: "Resolve", Dir: cwd},
		{Method: "CopyFrom", Dir: cwd, Paths: []string{"generated"}},
		{Method: "Restart", Dir: root, Services: []string{phpService}},
		{Method: "Capture", Dir: root, Service: phpService, Command: []string{"php", "-v"}},
		{Method: "Capture", Dir: root, Service: phpService, Command: []string{"php", "-i"}},
	}

	if diff := cmp.Diff(want, fake.calls, cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")); diff != "" {
		t.Fatalf("asked of the engine on macOS (-want +got):\n%s", diff)
	}

	printed := stdout.String()

	if !strings.Contains(printed, "Copying generated code into host") {
		t.Fatalf("stdout = %q, want the warning line before the copy", printed)
	}

	if !strings.Contains(printed, "xdebug.client_host") {
		t.Fatalf("stdout = %q, want the v3 settings read back", printed)
	}
}

// TestAFailedDebugOnCaptureIsReported proves both Capture calls are wrapped through failure()
// with a named step, whether they answer a non-zero status or a Go-level error — unlike the
// restart's own raw-passthrough status — and that a failing first call never lets a second one
// happen.
func TestAFailedDebugOnCaptureIsReported(t *testing.T) {
	root := "/code/shop"
	host := "172.17.0.1"

	baseOutcomes := func() []outcome {
		outcomes := []outcome{{}, {captured: host}, {}, {}, {}}
		outcomes = append(outcomes, macCopyOutcomes()...)
		outcomes = append(outcomes, outcome{})

		return outcomes
	}

	t.Run("the first capture, status", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = append(baseOutcomes(), outcome{status: 7})
		wantLen := len(fake.outcomes)
		stderr := &strings.Builder{}

		code := debugOn(io.Discard, stderr, false)

		if code != 7 {
			t.Fatalf("debug-on with a failed first capture = %d, want the container's own 7", code)
		}

		if len(fake.calls) != wantLen {
			t.Fatalf("debug-on asked the engine %d things, want exactly %d — no second Capture",
				len(fake.calls), wantLen)
		}

		if !strings.Contains(stderr.String(), "Xdebug version") {
			t.Fatalf("stderr = %q, want a line naming the version read as the failing step", stderr.String())
		}
	})

	t.Run("the first capture, error", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = append(baseOutcomes(), outcome{err: errors.New("docker: no")})
		wantLen := len(fake.outcomes)

		code := debugOn(io.Discard, io.Discard, false)

		if code != exitDocker {
			t.Fatalf("debug-on with a first-capture error = %d, want exitDocker", code)
		}

		if len(fake.calls) != wantLen {
			t.Fatalf("debug-on asked the engine %d things, want exactly %d — no second Capture",
				len(fake.calls), wantLen)
		}
	})

	t.Run("the second capture, status", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = append(baseOutcomes(), outcome{captured: phpVWithXdebugV2}, outcome{status: 9})
		stderr := &strings.Builder{}

		code := debugOn(io.Discard, stderr, false)

		if code != 9 {
			t.Fatalf("debug-on with a failed second capture = %d, want the container's own 9", code)
		}

		if !strings.Contains(stderr.String(), "settings") {
			t.Fatalf("stderr = %q, want a line naming the settings read as the failing step", stderr.String())
		}
	})

	t.Run("the second capture, error", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		fake.outcomes = append(baseOutcomes(),
			outcome{captured: phpVWithXdebugV3}, outcome{err: errors.New("docker: no")})

		code := debugOn(io.Discard, io.Discard, false)

		if code != exitDocker {
			t.Fatalf("debug-on with a second-capture error = %d, want exitDocker", code)
		}
	})
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
