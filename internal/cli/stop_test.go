package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What stop asks of the engine. The handler (orchestrate.go:44) separates --snapshot from the
// services, resolves the project and asks for one Stop; what a stop is — the snapshot first and
// nothing stopped if it fails, the worktree guard, Compose's own stop — is app.Operator's, and is
// tested there. Dir is the working directory: stop passes here(), not the resolved root.
//

func TestWhatStopAsks(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		services []string
		snapshot bool
	}{
		{"no services stops the whole environment", nil, []string{}, false},
		{"services are passed on in order", []string{"nginx", "phpfpm"}, []string{"nginx", "phpfpm"}, false},
		{"--snapshot asks for the copy first", []string{"--snapshot"}, []string{}, true},
		{"--snapshot is not a service, wherever it is", []string{"db", "--snapshot", "redis"}, []string{"db", "redis"}, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			if code := stop(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("stop %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Stop", Dir: cwd, Services: tt.services, Snapshot: tt.snapshot},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStopOutsideAProjectAsksNothing: the project is resolved first, and a directory that is not
// one stops nothing.
func TestStopOutsideAProjectAsksNothing(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{}

	if code := stop(nil, io.Discard, io.Discard, false); code != exitProject {
		t.Fatalf("stop outside a project = %d, want exitProject", code)
	}

	for _, c := range fake.calls {
		if c.Method == "Stop" {
			t.Fatalf("asked the engine to stop outside a project: %v", fake.calls)
		}
	}
}

// TestAStopRefusalKeepsItsContract: a refusal from the engine — the worktree guard — reaches the
// caller with its own code and type, not flattened into a generic failure.
func TestAStopRefusalKeepsItsContract(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind: "blocked_in_worktree", Code: 6, Message: "refused", Hint: "hm stop --force",
	}}}

	var stderr bytes.Buffer
	if code := stop(nil, io.Discard, &stderr, true); code != 6 {
		t.Fatalf("stop refused = %d, want 6", code)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Error.Type != "blocked_in_worktree" {
		t.Errorf("error type = %q, want blocked_in_worktree", envelope.Error.Type)
	}
}

// TestAFailedStopIsNotASuccess: any other error from the engine is a failure, never exit 0.
func TestAFailedStopIsNotASuccess(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: errors.New("compose stop failed")}}

	if code := stop(nil, io.Discard, io.Discard, false); code == exitOK {
		t.Fatal("stop with a failing engine = exitOK, want a failure")
	}
}
