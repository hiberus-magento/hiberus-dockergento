package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What down asks of the engine, and how it reports the answer. down_test.go pins what it refuses
// while parsing; this file pins what it asks once the arguments are good. What a down is — the
// question before deleting volumes, the snapshot first, the worktree guard — is app.Operator's.
// Dir is the working directory: down passes here(), not the resolved root.
//

func TestWhatDownAsks(t *testing.T) {
	thirty := 30

	cases := []struct {
		name    string
		args    []string
		options core.DownOptions
	}{
		{"no options removes the containers only", nil, core.DownOptions{}},
		{"-v deletes the volumes too", []string{"-v"}, core.DownOptions{Volumes: true}},
		{"--volumes is the same", []string{"--volumes"}, core.DownOptions{Volumes: true}},
		{"every option is passed on", []string{"--remove-orphans", "--rmi", "all", "-t", "30", "-v"},
			core.DownOptions{Volumes: true, RemoveOrphans: true, Images: "all", Timeout: &thirty}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			fake.outcomes = []outcome{{}, {captured: "destroyed"}}
			cwd := here()

			if code := down(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("down %v = %d, want exitOK", tt.args, code)
			}

			// answering(t) sets HM_NON_INTERACTIVE, so the question before deleting may not be asked
			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Down", Dir: cwd, DownOptions: tt.options, Interactive: false},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDownMayAskWithSomebodyThere: with nobody saying otherwise, the question may be asked.
func TestDownMayAskWithSomebodyThere(t *testing.T) {
	fake := answering(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {captured: "destroyed"}}

	if code := down([]string{"-v"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("down -v = %d, want exitOK", code)
	}

	if got := fake.calls[len(fake.calls)-1]; got.Method != "Down" || !got.Interactive {
		t.Fatalf("last call = %+v, want an interactive Down", got)
	}
}

// TestDownSaysWhenNothingWasDestroyed: somebody answering no is not a failure, and is said.
func TestDownSaysWhenNothingWasDestroyed(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {captured: ""}}

	var stdout bytes.Buffer
	if code := down([]string{"-v"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("down declined = %d, want exitOK", code)
	}

	if !strings.Contains(stdout.String(), "Nothing was destroyed.") {
		t.Errorf("stdout = %q, want it to say nothing was destroyed", stdout.String())
	}
}

// TestDownReportsWhatHappened: under --json, whether volumes went and whether a copy was taken.
func TestDownReportsWhatHappened(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		what     string
		volumes  bool
		snapshot bool
	}{
		{"destroyed without volumes", nil, "destroyed", false, false},
		{"destroyed with volumes, no copy", []string{"-v"}, "destroyed", true, false},
		{"a copy first, then destroyed", []string{"-v"}, "saved", true, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			fake.outcomes = []outcome{{}, {captured: tt.what}}

			var stdout bytes.Buffer
			if code := down(tt.args, &stdout, io.Discard, true); code != exitOK {
				t.Fatalf("down %v = %d, want exitOK", tt.args, code)
			}

			var envelope struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
			}

			want := map[string]any{"destroyed": true, "volumes": tt.volumes, "snapshot": tt.snapshot}
			if diff := cmp.Diff(want, envelope.Data); diff != "" {
				t.Errorf("data (-want +got):\n%s", diff)
			}
		})
	}
}

// TestADownRefusalKeepsItsContract: the worktree guard's refusal keeps its code.
func TestADownRefusalKeepsItsContract(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind: "blocked_in_worktree", Code: 6, Message: "refused", Hint: "hm down --force",
	}}}

	if code := down(nil, io.Discard, io.Discard, true); code != 6 {
		t.Fatalf("down refused = %d, want 6", code)
	}
}
