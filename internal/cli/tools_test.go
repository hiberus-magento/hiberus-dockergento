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
// What bash and masquerade ask of the engine. No t.Parallel anywhere in this file: t.Setenv and
// t.Chdir both panic in a parallel test or one with a parallel parent, and the decline test's
// askingWith call swaps process-global streams, a third reason.
//
// RED-window safety for bash, named rather than assumed: shell() (tools.go:10-18) only ever calls
// execute(), so bash's RED window is bounded the same way exec's is — by t.Chdir(t.TempDir())
// (fake_engine_test.go:274), not DOCKER_HOST. Before orchestrate.go:188 is routed, the unrouted
// call reaches Engine.Exec -> Resolve (succeeds with a nameless project) -> orchestrator.Exec ->
// open -> load, which refuses for want of a compose file under the temp root before service.Exec
// ever runs. A /nonexistent socket error here instead of "no compose file to read in <temp cwd>"
// is not an alarm by itself — see exec_test.go's header for what it would mean.
//
// RED-window safety for masquerade, the highest-consequence window in this change: an unrouted
// Engine.Anonymise (dockergento.go:452) call reaches Resolve (same nameless success) ->
// app.Importer.Anonymise (app/anonymise.go:26-30) -> Database.Ready (app/database.go:48) ->
// container (:121-145) -> Engine.Containers() — a machine-wide ContainerList that dials the
// daemon regardless of cwd. t.Chdir does not bound this window — DOCKER_HOST=unix:///nonexistent
// (fake_engine_test.go:272) is the only thing between an unrouted RED run and a real daemon. A
// second, independent layer exists too: app/database.go:128 matches on candidate.Key(), which
// falls back to ComposeProject when hm.project is empty (core/environment.go:112-118), and
// Compose stamps com.docker.compose.project on everything it creates, so a nameless project
// matches essentially nothing — but DOCKER_HOST stays load-bearing regardless. Every subtest
// that exercises masquerade runs under answering(t), and none overrides DOCKER_HOST.
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

// TestWhatMasqueradeAsks asserts what a confirmed masquerade asks of the engine: Resolve first,
// then Anonymise — confirmed by answering(t)'s own pin, which answers ask()'s suggestion ("Y")
// without reading stdin.
//
// Dir is the test's working directory, not f.project.Root: masquerade() (tools.go:54) reaches
// its engine call with here(), never with the project projectOr has just resolved. This
// function's handler passes here(), so it declares no root identifier: reaching for Dir: root
// here would not compile.
func TestWhatMasqueradeAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	code := masquerade(nil, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("masquerade = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Anonymise", Dir: cwd},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestMasqueradeRefusesADashedArgument pins tools.go:25-30's guard: any argument that starts
// with a dash is refused before an engine is built, the same shape down_test.go's refusals have.
func TestMasqueradeRefusesADashedArgument(t *testing.T) {
	cases := []string{"--nope", "-y"}

	for _, argument := range cases {
		t.Run(argument, func(t *testing.T) {
			fake := answering(t)

			stderr := &strings.Builder{}

			code := masquerade([]string{argument}, io.Discard, stderr, true)

			if code != exitUsage {
				t.Fatalf("masquerade %s = %d, want exitUsage", argument, code)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("masquerade %s asked the engine %v, want nothing asked at all", argument, fake.calls)
			}

			var envelope struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Type != "invalid_argument" {
				t.Fatalf("masquerade %s error.type = %q, want invalid_argument", argument, envelope.Error.Type)
			}
		})
	}
}

// TestAFailedAnonymiseIsReported is the branch where Anonymise itself fails: report() (tools.go:
// 54) maps it the same way every other handler's failure is mapped, and the call log still ends
// at Anonymise — Resolve succeeded before it.
func TestAFailedAnonymiseIsReported(t *testing.T) {
	failed := errors.New("anonymise: no")

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: failed}}
	cwd := here()

	stderr := &strings.Builder{}

	code := masquerade(nil, io.Discard, stderr, true)

	if code != exitDocker {
		t.Fatalf("masquerade with a failed Anonymise = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Anonymise", Dir: cwd},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine after a failed Anonymise (-want +got):\n%s", diff)
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
		t.Fatalf("masquerade with a failed Anonymise error.type = %q, want docker_failed", envelope.Error.Type)
	}
}

// TestADeclinedMasqueradeAsksOnlyToResolve has no RED half, and is written directly in this
// GREEN step: tools.go:51 returns exitOK before the engine call it declines, and projectOr is
// already routed, so the call log is [Resolve] whether tools.go:54 is routed through newEngine
// or not. Presenting this as a red-turning-green pair would be false — it proves a branch, not a
// routing.
//
// Declining is the branch answering(t) cannot reach on its own: it pins HM_NON_INTERACTIVE, and
// ask() (ask.go:29-35) answers with the suggestion without reading stdin while it is set, so the
// switch at tools.go:48-52 could only ever see "Y". A nested t.Setenv("HM_NON_INTERACTIVE", "")
// stacks over the helper's own pin here — docker_stop_all_test.go:21-24 does the same thing for
// the same reason: answering(t) pins HM_NON_INTERACTIVE for its RED-safe default; this test is
// about the interactive path, so it clears the pin back to what a caller's own terminal looks
// like. t.Setenv unwinds LIFO, so the pin is restored for every later test.
func TestADeclinedMasqueradeAsksOnlyToResolve(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	t.Setenv("HM_NON_INTERACTIVE", "")

	cwd := here()

	var code int

	// askingWith (ask_test.go:25) swaps the three standard streams for pipes and feeds the
	// answer into os.Stdin, which is what ask.go:45 reads — the writer masquerade() was handed
	// is not consulted, so nothing short of moving the global reaches the decline branch.
	_, stderr := askingWith(t, "n\n", func() {
		code = masquerade(nil, io.Discard, io.Discard, false)
	})

	// A fixture guard, not evidence about the branch: it catches exitProject from a project
	// whose Name never got set, and it runs before the diff so a broken fixture does not read as
	// a broken branch. It cannot tell declining from anonymising on its own — with no outcomes
	// configured the fake's Anonymise returns nil and report() answers exitOK too, so both sides
	// of an inverted default: branch are exitOK. The log diff below is what discriminates.
	if code != exitOK {
		t.Fatalf("masquerade declined = %d, want exitOK", code)
	}

	// The assertion that actually discriminates: an Anonymise entry here means the answer was
	// not read.
	want := []call{{Method: "Resolve", Dir: cwd}}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine after declining (-want +got):\n%s", diff)
	}

	// The log alone cannot tell "declined" from "returned early without asking": both leave
	// exactly [Resolve]. This is the assertion that distinguishes them.
	if !strings.Contains(stderr, "anonymise your database") {
		t.Fatalf("stderr = %q, want the question that was answered", stderr)
	}
}
