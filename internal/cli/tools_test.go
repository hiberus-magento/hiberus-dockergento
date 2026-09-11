package cli

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What bash asks of the engine (masquerade's tests join this file in a later unit). No
// t.Parallel anywhere in this file: t.Setenv and t.Chdir both panic in a parallel test or one
// with a parallel parent, and every subtest here calls one or the other through answering(t).
//
// RED-window safety, named rather than assumed: shell() (tools.go:10-18) only ever calls
// execute(), so bash's RED window is bounded the same way exec's is — by t.Chdir(t.TempDir())
// (fake_engine_test.go:274), not DOCKER_HOST. Before orchestrate.go:188 is routed, the unrouted
// call reaches Engine.Exec -> Resolve (succeeds with a nameless project) -> orchestrator.Exec ->
// open -> load, which refuses for want of a compose file under the temp root before service.Exec
// ever runs. A /nonexistent socket error here instead of "no compose file to read in <temp cwd>"
// is not an alarm by itself — see exec_test.go's header for what it would mean.
//

// TestWhatBashAsks asserts what bash asks of the engine: Resolve first, then Exec with a fixed
// ["bash"] command, and that -r selects the root user without changing that command.
//
// Dir is the test's working directory, not f.project.Root: shell() reaches execute(), which asks
// about here() (os.Getwd()) and never about the project projectOr has just resolved.
// inside_test.go asserts Dir: root for the opposite reason — inside() passes project.Root — so
// this is where the two precedents diverge. This function's handler passes here(), so it
// declares no root identifier: reaching for Dir: root here would not compile.
func TestWhatBashAsks(t *testing.T) {
	ignoreTty := cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")

	cases := []struct {
		name     string
		args     []string
		wantUser string
	}{
		{"no arguments", nil, ""},
		{"-r opens a root shell", []string{"-r"}, "root"},
		{"-c is silently discarded", []string{"-c", "echo hi"}, ""},
		{"-r -c honours only the leading -r", []string{"-r", "-c", "echo hi"}, "root"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			code := shell(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("bash %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Exec", Dir: cwd, Service: phpService,
					Command: []string{"bash"}, Options: terminalOptions(tt.wantUser)},
			}

			if diff := cmp.Diff(want, fake.calls, ignoreTty); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}
