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
// What rebuild asks of the engine: the project resolved, then one Rebuild with the services it was
// given. What a rebuild is — stop, up --build, the bind-mount check and the steps after starting —
// is app.Operator.Rebuild's, and is tested there.
//

func TestWhatRebuildAsks(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		services []string
	}{
		{"no services rebuilds the whole environment", nil, []string{}},
		{"the services are passed on in order", []string{"phpfpm", "nginx"}, []string{"phpfpm", "nginx"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			// Through the router: a case in run.go's switch, not the fallthrough to the shell
			if code := Run(append([]string{"rebuild"}, tt.args...), io.Discard, io.Discard); code != exitOK {
				t.Fatalf("rebuild %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Rebuild", Dir: cwd, Services: tt.services},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRebuildRefusesAnOption(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	if code := rebuild([]string{"phpfpm", "--no-cache"}, io.Discard, io.Discard, false); code != exitUsage {
		t.Fatalf("rebuild --no-cache = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("rebuild --no-cache asked the engine %v, want nothing", fake.calls)
	}
}

func TestRebuildAnswersWithTheRefusalsOwnCode(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "blocked_in_worktree",
		Code:    exitBlocked,
		Message: "'rebuild' is refused from a worktree",
		Hint:    "Run it from /code/shop, or repeat with --force",
	}}}

	stderr := &strings.Builder{}

	if code := rebuild(nil, io.Discard, stderr, true); code != exitBlocked {
		t.Fatalf("a refused rebuild = %d, want exitBlocked", code)
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
