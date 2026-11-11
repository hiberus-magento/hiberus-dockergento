package cli

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What restart asks of the engine. The handler (orchestrate.go:65) parses its arguments, resolves
// the project and asks for one Restart with the services it was given; what a restart is — stop,
// then start, and on Linux the post-start hand-back — is app.Operator.Restart's, and is tested
// there. Dir is the working directory: restart passes here(), not the resolved root.
//

func TestWhatRestartAsks(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		services []string
	}{
		{"no services restarts the whole environment", nil, []string{}},
		{"one service is passed on", []string{"phpfpm"}, []string{"phpfpm"}},
		{"several are passed on in order", []string{"nginx", "phpfpm"}, []string{"nginx", "phpfpm"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			code := restart(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("restart %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Restart", Dir: cwd, Services: tt.services},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRestartRefusesAnOption pins the dash guard (orchestrate.go:68-72): an option is a usage
// error before anything is resolved, not a service name handed to Compose.
func TestRestartRefusesAnOption(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	code := restart([]string{"-s"}, io.Discard, io.Discard, false)

	if code != exitUsage {
		t.Fatalf("restart -s = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("restart -s asked the engine %v, want nothing", fake.calls)
	}
}

// TestRestartAnswersWithTheRefusalsOwnCode is what a set -e caller sees: a refused restart — from
// a worktree with no environment of its own, say — exits with the refusal's code and says so
// under --json, instead of reporting success.
func TestRestartAnswersWithTheRefusalsOwnCode(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "blocked_in_worktree",
		Code:    exitBlocked,
		Message: "'restart' is refused from a worktree",
		Hint:    "Run it from /code/shop, or repeat with --force",
	}}}

	stderr := &strings.Builder{}

	code := restart([]string{"phpfpm"}, io.Discard, stderr, true)

	if code != exitBlocked {
		t.Fatalf("a refused restart = %d, want exitBlocked", code)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Error.Type != "blocked_in_worktree" {
		t.Errorf("error type = %q, want blocked_in_worktree", envelope.Error.Type)
	}
}

func TestRestartOutsideAProjectAsksNothingMore(t *testing.T) {
	fake := answering(t)
	cwd := here()

	code := restart(nil, io.Discard, io.Discard, false)

	if code != exitProject {
		t.Fatalf("restart outside a project = %d, want exitProject", code)
	}

	if diff := cmp.Diff([]call{{Method: "Resolve", Dir: cwd}}, fake.calls); diff != "" {
		t.Fatalf("asked of the engine outside a project (-want +got):\n%s", diff)
	}
}
