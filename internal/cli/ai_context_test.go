package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// shellAIContext runs console/commands/ai-context.sh through the oracle harness in a directory
// seeded with files, and answers with what it printed and the result its --json document carried.
//
// TEMPORARY SCAFFOLDING, removed with console/tasks/agent_context.sh: see
// dockergento/core/testdata/ai_context_oracle.sh.
func shellAIContext(t *testing.T, format string, files map[string]string) (stdout, stderr string) {
	t.Helper()

	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("the shell implementation needs jq")
	}

	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	root := filepath.Join(work, "shop")

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	info := `{"project":{"name":"shop","domain":"shop.test","status":"stopped",` +
		`"urls":{"base":"https://shop.test/","admin":"https://shop.test/admin"}},` +
		`"magento":{"version":"2.4.7","mode":"developer"},"data":{"anonymised":"unknown"},` +
		`"services":[{"name":"phpfpm","image":"hiberusmagento/php:8.3-fpm"}]}`

	if err := os.WriteFile(filepath.Join(work, "info.json"), []byte(info), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("bash", filepath.Join(repository, "dockergento", "core", "testdata", "ai_context_oracle.sh"))
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + work,
		"ORACLE_REPO=" + repository,
		"ORACLE_WORK=" + work,
		"ORACLE_INFO=" + filepath.Join(work, "info.json"),
		"HM_ROOT=" + root,
		"HM_STATE_DIR=" + filepath.Join(work, "state"),
		"COMPOSE_PROJECT_NAME=shop",
		"COMMAND_BIN_NAME=hm",
		"HM_OUTPUT_FORMAT=" + format,
	}

	var out, errs bytes.Buffer
	command.Stdout, command.Stderr = &out, &errs

	if err := command.Run(); err != nil {
		t.Fatalf("the shell implementation failed: %v\n%s", err, errs.String())
	}

	return out.String(), errs.String()
}

// TestAIContextPrintsWhatTheShellPrinted compares the handler's output with the script's, in
// both formats, for the two shapes it has: CLAUDE.md created, and CLAUDE.md somebody else's.
//
// TEMPORARY SCAFFOLDING, kept while console/commands/ai-context.sh exists and removed with it.
func TestAIContextPrintsWhatTheShellPrinted(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"nothing there yet", nil},
		{"a CLAUDE.md of somebody's", map[string]string{"CLAUDE.md": "Mine.\n"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shellJSON, _ := shellAIContext(t, "json", c.files)
			shellText, shellErrors := shellAIContext(t, "text", c.files)

			var answered struct {
				Data core.AgentContextResult `json:"data"`
			}
			if err := json.Unmarshal([]byte(shellJSON), &answered); err != nil {
				t.Fatalf("the shell implementation's answer is not JSON: %v\n%s", err, shellJSON)
			}

			// The same result, so that the only thing compared is how it is printed. Its root is
			// the shell run's directory, which is in the document
			aiContextFor(t, answered.Data)

			var stdout, stderr bytes.Buffer
			if code := aiContext(nil, &stdout, &stderr, true); code != exitOK {
				t.Fatalf("ai-context --json = %d, want exitOK\n%s", code, stderr.String())
			}

			if diff := cmp.Diff(shellJSON, stdout.String()); diff != "" {
				t.Errorf("the --json document differs (-shell +go):\n%s", diff)
			}

			stdout.Reset()
			stderr.Reset()

			if code := aiContext(nil, &stdout, &stderr, false); code != exitOK {
				t.Fatalf("ai-context = %d, want exitOK\n%s", code, stderr.String())
			}

			if diff := cmp.Diff(shellText, stdout.String()); diff != "" {
				t.Errorf("stdout differs (-shell +go):\n%s", diff)
			}

			if diff := cmp.Diff(shellErrors, stderr.String()); diff != "" {
				t.Errorf("stderr differs (-shell +go):\n%s", diff)
			}
		})
	}
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
