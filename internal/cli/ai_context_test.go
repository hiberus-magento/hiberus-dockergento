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

// aiContextFor is a fake that resolves a project and answers WriteAgentContext with result.
func aiContextFor(t *testing.T, result core.AgentContextResult) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.agentContext = result
	t.Setenv("NO_COLOR", "1")

	return fake
}

func TestAIContextAsksTheEngineAboutThisDirectory(t *testing.T) {
	fake := aiContextFor(t, core.AgentContextResult{Written: []string{"AGENTS.md"}, Root: "/code/shop"})

	if code := aiContext(nil, io.Discard, io.Discard, true); code != exitOK {
		t.Fatalf("ai-context = %d, want exitOK", code)
	}

	want := []call{{Method: "Resolve", Dir: here()}, {Method: "WriteAgentContext", Dir: here()}}
	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

// TestAIContextRefusesAnOptionAndOnlyTheFirst: the script looked at its first argument and no
// further, and an empty one was no argument at all.
func TestAIContextRefusesAnOptionAndOnlyTheFirst(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"--force-it"}, exitUsage},
		{[]string{"", "ignored"}, exitOK},
	}

	for _, c := range cases {
		fake := aiContextFor(t, core.AgentContextResult{})

		var stderr bytes.Buffer
		if code := aiContext(c.args, io.Discard, &stderr, true); code != c.want {
			t.Errorf("ai-context %q = %d, want %d\n%s", c.args, code, c.want, stderr.String())
		}

		if c.want == exitUsage && len(fake.calls) != 0 {
			t.Errorf("ai-context %q asked %v, want nothing", c.args, fake.calls)
		}
	}
}

// TestAIContextReportsWhatTheEngineRefused: the code and the kind are the refusal's own — a block
// nobody closed is refused on purpose — and a file that could not be written is not Docker's.
func TestAIContextReportsWhatTheEngineRefused(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		kind string
	}{
		{"an unclosed block", core.Refusal{Kind: "block_not_closed", Code: exitBlocked, Message: "AGENTS.md has an opening marker"},
			exitBlocked, "block_not_closed"},
		{"no daemon", core.Refusal{Kind: "docker_unavailable", Code: exitDocker, Message: "Docker is not running"},
			exitDocker, "docker_unavailable"},
		{"a write that failed", errors.New("permission denied"), exitError, "write_failed"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := aiContextFor(t, core.AgentContextResult{})
			fake.outcomes = []outcome{{}, {err: c.err}}

			var stdout, stderr bytes.Buffer
			if code := aiContext(nil, &stdout, &stderr, true); code != c.code {
				t.Fatalf("ai-context = %d, want %d", code, c.code)
			}

			var document struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &document); err != nil || document.Error.Type != c.kind {
				t.Errorf("stderr = %s, want a %s envelope", stderr.String(), c.kind)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}
		})
	}
}

func TestAIContextOutsideAProjectIsRefused(t *testing.T) {
	fake := answering(t)

	if code := aiContext(nil, io.Discard, io.Discard, true); code != exitProject {
		t.Fatalf("ai-context = %d, want exitProject", code)
	}

	if len(fake.calls) != 1 {
		t.Errorf("asked %v, want only Resolve", fake.calls)
	}
}

// TestTheRouterAnswersForAIContext: it reaches its handler, and its help stays the shell's.
func TestTheRouterAnswersForAIContext(t *testing.T) {
	fake := aiContextFor(t, core.AgentContextResult{})

	Run([]string{"ai-context"}, io.Discard, io.Discard)

	if len(fake.calls) == 0 || fake.calls[len(fake.calls)-1].Method != "WriteAgentContext" {
		t.Fatalf("ai-context fell through to the shell implementation: %v", fake.calls)
	}

	// answering(t) points the legacy runner at a directory with no bin/run, so reaching it fails
	// with 3 — where the handler would have refused the option with 2
	for _, help := range []string{"--help", "-h"} {
		var stderr bytes.Buffer
		if code := Run([]string{"ai-context", help}, io.Discard, &stderr); code != 3 {
			t.Errorf("ai-context %s = %d (%s), want it handed to the shell implementation's help",
				help, code, stderr.String())
		}
	}
}
