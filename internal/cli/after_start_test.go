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
// What post-start asks of the engine: the project resolved, then one AfterStart for the working
// directory. What the steps are — and that on macOS there are none — is app.Operator.AfterStart's,
// and is tested there.
//

func TestWhatPostStartAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	// What follows the command is ignored, as post-start.sh ignored it
	if code := Run([]string{"post-start", "ignored"}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("post-start = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "AfterStart", Dir: cwd},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestPostStartAnswersWithTheRefusalsOwnCode(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "usermod_missing",
		Code:    exitError,
		Message: "Commands usermod and groupmod are required",
	}}}

	stderr := &strings.Builder{}

	if code := postStart(io.Discard, stderr, true); code != exitError {
		t.Fatalf("a refused post-start = %d, want exitError", code)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Error.Type != "usermod_missing" {
		t.Errorf("error type = %q, want usermod_missing", envelope.Error.Type)
	}
}

func TestPostStartOutsideAProjectAsksNothingMore(t *testing.T) {
	fake := answering(t)
	cwd := here()

	if code := postStart(io.Discard, io.Discard, false); code != exitProject {
		t.Fatalf("post-start outside a project = %d, want exitProject", code)
	}

	if diff := cmp.Diff([]call{{Method: "Resolve", Dir: cwd}}, fake.calls); diff != "" {
		t.Fatalf("asked of the engine outside a project (-want +got):\n%s", diff)
	}
}
