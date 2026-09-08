package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What docker-stop-all asks of the engine: exactly one request to stop everything, for the
// resolved directory, carrying whether the run is interactive. Whatever the fake answers back —
// a decline, a refusal, or a stop — is reported without the handler asking anything else.
//

func TestWhatDockerStopAllAsks(t *testing.T) {
	fake := answering(t)
	// answering(t) pins HM_NON_INTERACTIVE for its RED-safe default; this test is about the
	// interactive path, so it clears the pin back to what a caller's own terminal looks like.
	t.Setenv("HM_NON_INTERACTIVE", "")
	fake.stopped = core.MachineStop{Total: 3, Others: 1, Stopped: 3}
	cwd := here()

	code := dockerStopAll(nil, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("docker-stop-all = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "StopEverything", Dir: cwd, Interactive: true},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestWhatDockerStopAllAsksNonInteractively(t *testing.T) {
	fake := answering(t)
	t.Setenv("HM_NON_INTERACTIVE", "1")
	cwd := here()

	code := dockerStopAll(nil, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("docker-stop-all --yes = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "StopEverything", Dir: cwd, Interactive: false},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestAFailedStopIsReportedAsADockerFailure is the CLI side of "some containers refuse to stop":
// the app names them in the error, and report() turns that into exitDocker/docker_failed, the way
// every other engine refusal that is not a core.Refusal already does.
func TestAFailedStopIsReportedAsADockerFailure(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{}, {err: errors.New("could not stop: b")}}

	stderr := &strings.Builder{}

	code := dockerStopAll(nil, io.Discard, stderr, true)

	if code != exitDocker {
		t.Fatalf("docker-stop-all with a refusal = %d, want exitDocker", code)
	}

	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
	}

	if envelope.Error.Type != "docker_failed" {
		t.Fatalf("error type = %q, want docker_failed", envelope.Error.Type)
	}

	if !strings.Contains(envelope.Error.Message, "b") {
		t.Fatalf("message = %q, want the refusing container named", envelope.Error.Message)
	}
}

func TestDockerStopAllAnswersADocumentWhenNobodyIsWatching(t *testing.T) {
	fake := answering(t)
	fake.stopped = core.MachineStop{Total: 3, Others: 1, Stopped: 3}

	stdout := &strings.Builder{}

	code := dockerStopAll(nil, stdout, io.Discard, true)

	if code != exitOK {
		t.Fatalf("docker-stop-all --json = %d, want exitOK", code)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}

	if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
	}

	want := map[string]any{"total": float64(3), "others": float64(1), "stopped": true}

	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Fatalf("docker-stop-all --json data (-want +got):\n%s", diff)
	}
}
