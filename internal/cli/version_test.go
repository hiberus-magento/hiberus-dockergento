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
// What version asks of the engine — exactly one call, about no project at all, which is what
// makes this the shortest handler in the package to prove.
//

func TestWhatVersionReports(t *testing.T) {
	t.Run("--json", func(t *testing.T) {
		fake := answering(t)
		fake.installed = core.Installation{
			Version:      "1.4.5",
			Tag:          "v1.4.5",
			CommitsAhead: 3,
			Commit:       "abc1234",
			Branch:       "release/2.0.0",
			Detached:     false,
			Dirty:        true,
			Path:         "/opt/hm",
		}
		fake.tooling = core.Tooling{
			Docker: "24.0.5", Compose: "2.20.0", ComposeCommand: "docker compose",
		}

		stdout := &strings.Builder{}

		code := version(nil, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("version --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		binary, _ := envelope.Data["binary"].(string)
		delete(envelope.Data, "binary")

		if binary != buildOfThisBinary() {
			t.Fatalf("data.binary = %q, want %q", binary, buildOfThisBinary())
		}

		want := map[string]any{
			"version":       "1.4.5",
			"tag":           "v1.4.5",
			"commits_ahead": float64(3),
			"commit":        "abc1234",
			"branch":        "release/2.0.0",
			"detached":      false,
			"dirty":         true,
			"path":          "/opt/hm",
			"docker": map[string]any{
				"version": "24.0.5", "compose": "2.20.0", "compose_command": "docker compose",
			},
		}

		if diff := cmp.Diff(want, envelope.Data); diff != "" {
			t.Fatalf("version --json data (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]call{{Method: "Installed"}}, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	// A detached checkout with nothing underneath is the only case that reaches both orUnknown
	// (an empty tag, an empty commit) and orMissing (an empty docker and compose value), down to
	// the "hm switch --list" footer.
	t.Run("a detached checkout with nothing underneath, in text", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("COMMAND_BIN_NAME", "hm")

		fake := answering(t)
		fake.installed = core.Installation{
			Version:  "1.4.5",
			Detached: true,
			Path:     "/opt/hm",
		}

		stdout := &strings.Builder{}

		code := version(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("version = %d, want exitOK", code)
		}

		want := "" +
			"hm 1.4.5\n" +
			"  version      unknown (detached checkout)\n" +
			"  commit       unknown\n" +
			"  installed    /opt/hm\n" +
			"\n" +
			"  docker       not available\n" +
			"  compose      not available\n" +
			"  binary       " + buildOfThisBinary() + "\n" +
			"\n" +
			"  hm switch --list   to see the versions available\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("version text (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff([]call{{Method: "Installed"}}, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})
}

// version has nothing to ask about with an argument: it is the shortest path in the CLI, and an
// argument it never declared is a usage error rather than something it forwards.
func TestAnArgumentNobodyDeclaredIsAUsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"an unknown option", []string{"--tonteria"}},
		{"a bare positional", []string{"extra"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			code := version(tt.args, io.Discard, io.Discard, false)

			if code != exitUsage {
				t.Fatalf("version %v = %d, want exitUsage", tt.args, code)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("version %v asked the engine %v, want nothing asked at all", tt.args, fake.calls)
			}
		})
	}
}
