package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What start asks of the engine. The handler (orchestrate.go:15) separates -s from the services,
// refuses any other option, resolves the project and asks for one Start; what a start is — the
// worktree guard, stopping the rest, the proxy, Compose's up, the bind-mount check and Linux's
// post-start — is app.Operator.Start's, and is tested there. Dir is the working directory.
//

func TestWhatStartAsks(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		services   []string
		stopOthers bool
	}{
		{"no services starts the whole environment", nil, []string{}, false},
		{"services are passed on in order", []string{"phpfpm", "nginx"}, []string{"phpfpm", "nginx"}, false},
		{"-s stops the rest first", []string{"-s"}, []string{}, true},
		{"-s is not a service, wherever it is", []string{"db", "-s"}, []string{"db"}, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			if code := start(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("start %v = %d, want exitOK", tt.args, code)
			}

			// answering(t) sets HM_NON_INTERACTIVE, so the stop of the rest may not ask
			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Start", Dir: cwd, Services: tt.services, StopOthers: tt.stopOthers, Interactive: false},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStartMayAskWithSomebodyThere: with nobody saying otherwise, stopping the rest may ask first.
func TestStartMayAskWithSomebodyThere(t *testing.T) {
	fake := answering(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	if code := start([]string{"-s"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("start -s = %d, want exitOK", code)
	}

	if got := fake.calls[len(fake.calls)-1]; got.Method != "Start" || !got.Interactive {
		t.Fatalf("last call = %+v, want an interactive Start", got)
	}
}

// TestStartRefusesAnOption: an option other than -s is a usage error before anything is resolved,
// not a service name handed to Compose.
func TestStartRefusesAnOption(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	var stderr bytes.Buffer
	if code := start([]string{"--build"}, io.Discard, &stderr, true); code != exitUsage {
		t.Fatalf("start --build = %d, want exitUsage", code)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Error.Type != "invalid_argument" {
		t.Errorf("error type = %q, want invalid_argument", envelope.Error.Type)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestAStartRefusalKeepsItsContract: the worktree guard's refusal reaches the caller with its own
// code and type.
func TestAStartRefusalKeepsItsContract(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind: "blocked_in_worktree", Code: 6, Message: "refused", Hint: "hm start --force",
	}}}

	if code := start(nil, io.Discard, io.Discard, true); code != 6 {
		t.Fatalf("start refused = %d, want 6", code)
	}
}
