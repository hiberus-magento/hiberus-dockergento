package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What exec asks of the engine. No t.Parallel anywhere in this file: t.Setenv and t.Chdir both
// panic in a parallel test or one with a parallel parent, and every subtest here calls one or the
// other through answering(t).
//
// RED-window safety, named rather than assumed: before orchestrate.go:188 is routed, execute()
// still calls the real engine() rather than newEngine(). That unrouted call reaches Engine.Exec ->
// Resolve, which succeeds with a nameless project outside a real repository (dockergento.go:
// 128-130 says so in its own comment) -> orchestrator.Exec (composelib/orchestrator.go:145) ->
// open (:166), which builds and Initializes a docker CLI, -> load (:204-215), which refuses for
// want of a compose file under the temp root and returns before service.Exec ever runs. What
// bounds this window is t.Chdir(t.TempDir()) (fake_engine_test.go:274), not DOCKER_HOST — nothing
// on this path dials before load refuses. If a /nonexistent socket error appears instead of "no
// compose file to read in <temp cwd>", that is not an alarm: it means Initialize dials after all,
// and the thing to check is whether load was reached, not whether t.Chdir held. Both paths still
// stop before service.Exec, so no reach is added either way.
//

// TestWhatExecAsks asserts what a resolved exec call asks of the engine: Resolve first, then Exec
// with the command and the user -r selects, in that order.
//
// Dir is the test's working directory, not f.project.Root: orchestrate.go:188 asks about here()
// (os.Getwd()) and never about the project projectOr has just resolved. inside_test.go asserts
// Dir: root for the opposite reason — inside() passes project.Root — so this is where the two
// precedents diverge. If this line ever fails, the want is right and the change to orchestrate.go
// is the defect. This function's handler passes here(), so it declares no root identifier:
// reaching for Dir: root here would not compile.
func TestWhatExecAsks(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")

	cases := []struct {
		name        string
		args        []string
		wantCommand []string
		wantUser    string
	}{
		{"a plain command", []string{"ls", "-lah"}, []string{"ls", "-lah"}, ""},
		{"-r first sets the root user", []string{"-r", "whoami"}, []string{"whoami"}, "root"},
		{"-r not first is left in the command", []string{"ls", "-r"}, []string{"ls", "-r"}, ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			code := execute(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("exec %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Exec", Dir: cwd, Service: phpService,
					Command: tt.wantCommand, Options: terminalOptions(tt.wantUser)},
			}

			if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestExecRefusesWithNoCommand pins orchestrate.go:179-182's guard, and that -r is stripped
// (:174-177) before it, so hm exec -r alone is missing_command too.
func TestExecRefusesWithNoCommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no arguments at all", nil},
		{"-r alone", []string{"-r"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			stderr := &strings.Builder{}

			code := execute(tt.args, io.Discard, stderr, true)

			if code != exitUsage {
				t.Fatalf("exec %v = %d, want exitUsage", tt.args, code)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("exec %v asked the engine %v, want nothing asked at all", tt.args, fake.calls)
			}

			var envelope struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Type != "missing_command" {
				t.Fatalf("exec %v error.type = %q, want missing_command", tt.args, envelope.Error.Type)
			}
		})
	}
}

// TestAFailedExecStatusIsReturnedAsIs is orchestrate.go:190-192's own branch: a non-zero status
// with no error returns as-is, without ever reaching report()'s docker_failed mapping — the
// branch that keeps a grep that matched nothing from turning into a Docker failure.
func TestAFailedExecStatusIsReturnedAsIs(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {status: 2}}

	code := execute([]string{"grep", "nope", "file"}, io.Discard, io.Discard, false)

	if code != 2 {
		t.Fatalf("exec with a failed status = %d, want the container's own 2", code)
	}
}

// TestExecReportsAFailure is the other half of orchestrate.go:190-194's branch: a zero status
// together with a non-nil error falls through into report(), unlike the status-only case above.
func TestExecReportsAFailure(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")
	refused := errors.New("docker: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {status: 0, err: refused}}
	cwd := here()

	stderr := &strings.Builder{}

	code := execute([]string{"ls"}, io.Discard, stderr, true)

	if code != exitDocker {
		t.Fatalf("exec with a failed call = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: cwd, Service: phpService,
			Command: []string{"ls"}, Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
		t.Fatalf("asked of the engine after a failed call (-want +got):\n%s", diff)
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
		t.Fatalf("exec with a failed call error.type = %q, want docker_failed", envelope.Error.Type)
	}
}
