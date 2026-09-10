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
// What list refuses before it ever reaches an engine. No fake, no answering(t): the refusal
// returns during parsing, before any engine call is made — the same shape down_test.go:11-18
// established. Both rows pin COMMAND_BIN_NAME explicitly, the version_test.go:86 precedent:
// bin/run:4 exports COMMAND_BIN_NAME="hm", so an ambient value would otherwise decide the default
// row by accident, and the renamed-binary row is what can only pass once binaryName()
// (engine.go:106-112) is genuinely called.
//
// RED-window safety, named rather than assumed: this machine can have other Hiberus Dockergento
// projects running with live containers. Environments() takes no directory argument at all
// (list.go:23 calls it with nothing), so it cannot be bounded by t.Chdir the way a project-scoped
// call could — it goes straight to app.Inventory{...}.Environments() (dockergento.go:140-146) ->
// dockerd.Engine.Containers() -> a Docker SDK ContainerList across the whole machine. Its only
// protection is the DOCKER_HOST=unix:///nonexistent pin (fake_engine_test.go:218), load-bearing
// because dockerd.Endpoint() gives DOCKER_HOST precedence over the context store and
// client.FromEnv reads it too. Every subtest below that exercises list runs under answering(t),
// and none overrides DOCKER_HOST.
//

func TestListRefusesAnyArgument(t *testing.T) {
	cases := []struct {
		name       string
		binaryName string
		args       []string
		wantHint   string
		withFake   bool
	}{
		{"the circular --help hint", "hm", []string{"--help"}, "hm list", false},
		{"a renamed binary", "hd", []string{"--tonteria"}, "hd list", false},
		{"a positional argument under a substituted engine", "hm", []string{"algo"}, "hm list", true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var fake *fakeEngine
			if tt.withFake {
				fake = answering(t)
			} else {
				t.Chdir(t.TempDir())
			}
			t.Setenv("COMMAND_BIN_NAME", tt.binaryName)

			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := list(tt.args, stdout, stderr, true)

			if code != exitUsage {
				t.Fatalf("list %v = %d, want exitUsage", tt.args, code)
			}

			var envelope struct {
				Error struct {
					Hint string `json:"hint"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Hint != tt.wantHint {
				t.Fatalf("list %v hint = %q, want %q", tt.args, envelope.Error.Hint, tt.wantHint)
			}

			// The row under a substituted engine also proves list.go:18-21 never lets a
			// positional through to it: a future loosening of that guard would show up here as
			// a non-empty call log, not just as a wrong hint.
			if tt.withFake && len(fake.calls) != 0 {
				t.Fatalf("list %v asked the engine %v, want nothing asked at all", tt.args, fake.calls)
			}
		})
	}
}

// TestWhatListAsks is what list asks of the engine: exactly one call, about no project at all,
// because Environments() takes no directory and list.go never calls projectOr.
func TestWhatListAsks(t *testing.T) {
	fake := answering(t)
	fake.environments = []core.Environment{{Name: "shop"}}

	code := list(nil, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("list = %d, want exitOK", code)
	}

	if diff := cmp.Diff([]call{{Method: "Environments"}}, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestWhatListPrints is what list renders as text. Every want below is a hand-written literal:
// never a fmt.Sprintf of list.go's own format string, which would compare the format with itself.
func TestWhatListPrints(t *testing.T) {
	t.Run("an empty inventory", func(t *testing.T) {
		fake := answering(t)
		fake.environments = nil

		stdout := &strings.Builder{}

		code := list(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("list = %d, want exitOK", code)
		}

		want := "\nNo Hiberus Dockergento environments found on this machine.\n\n" +
			"  Create one with hm setup inside a Magento project.\n\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("list text (-want +got):\n%s", diff)
		}
	})

	t.Run("one ordinary row", func(t *testing.T) {
		fake := answering(t)
		environment := core.Environment{
			Name:        "shop",
			Root:        "/code/shop",
			Branch:      "main",
			HasMetadata: true,
			Status:      "running",
		}
		environment.Containers.Running = 5
		environment.Containers.Total = 5
		fake.environments = []core.Environment{environment}

		stdout := &strings.Builder{}

		code := list(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("list = %d, want exitOK", code)
		}

		want := "\n" +
			"   PROJECT                    STATUS    SERVICE BRANCH                 ROOT\n" +
			"   -------------------------- --------- ------- ---------------------- ----\n" +
			"   shop                       running   5/5     main                   /code/shop\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("list text (-want +got):\n%s", diff)
		}
	})

	t.Run("an orphan row with no metadata and no branch or root", func(t *testing.T) {
		fake := answering(t)
		environment := core.Environment{
			Name:        "gone",
			Status:      "stopped",
			HasMetadata: false,
			Orphan:      true,
		}
		environment.Containers.Total = 3
		fake.environments = []core.Environment{environment}

		stdout := &strings.Builder{}

		code := list(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("list = %d, want exitOK", code)
		}

		want := "\n" +
			"   PROJECT                    STATUS    SERVICE BRANCH                 ROOT\n" +
			"   -------------------------- --------- ------- ---------------------- ----\n" +
			"   gone                       stopped   0/3     -                      -  ⚠ orphan  (no metadata)\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("list text (-want +got):\n%s", diff)
		}
	})

	t.Run("a row with a worktree", func(t *testing.T) {
		fake := answering(t)
		environment := core.Environment{
			Name:        "shop",
			Worktree:    "feature-x",
			Root:        "/code/shop-wt",
			Branch:      "develop",
			HasMetadata: true,
			Status:      "partial",
		}
		environment.Containers.Running = 2
		environment.Containers.Total = 4
		fake.environments = []core.Environment{environment}

		stdout := &strings.Builder{}

		code := list(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("list = %d, want exitOK", code)
		}

		want := "\n" +
			"   PROJECT                    STATUS    SERVICE BRANCH                 ROOT\n" +
			"   -------------------------- --------- ------- ---------------------- ----\n" +
			"   shop (wt: feature-x)       partial   2/4     develop                /code/shop-wt\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("list text (-want +got):\n%s", diff)
		}
	})
}

// TestWhatListReportsAsJSON decodes list --json against a hand-written literal, never a
// re-marshalled fixture — comparing the encoder with itself would pass through any tag rename.
func TestWhatListReportsAsJSON(t *testing.T) {
	fake := answering(t)

	shop := core.Environment{
		Name:        "shop",
		Root:        "/code/shop",
		Magento:     "2.4.6",
		Branch:      "main",
		HasMetadata: true,
		Status:      "running",
	}
	shop.Containers.Running = 3
	shop.Containers.Total = 3

	legacy := core.Environment{
		Name:        "legacy",
		HasMetadata: false,
		Orphan:      true,
		Status:      "stopped",
	}
	legacy.Containers.Running = 0
	legacy.Containers.Total = 2

	fake.environments = []core.Environment{shop, legacy}

	stdout := &strings.Builder{}

	code := list(nil, stdout, io.Discard, true)

	if code != exitOK {
		t.Fatalf("list --json = %d, want exitOK", code)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}

	if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
	}

	want := map[string]any{
		"environments": []any{
			map[string]any{
				"name":         "shop",
				"root":         "/code/shop",
				"worktree":     "",
				"magento":      "2.4.6",
				"has_metadata": true,
				"containers":   map[string]any{"running": float64(3), "total": float64(3)},
				"branch":       "main",
				"orphan":       false,
				"status":       "running",
			},
			map[string]any{
				"name":         "legacy",
				"root":         "",
				"worktree":     "",
				"magento":      "",
				"has_metadata": false,
				"containers":   map[string]any{"running": float64(0), "total": float64(2)},
				"branch":       "",
				"orphan":       true,
				"status":       "stopped",
			},
		},
		"count": float64(2),
	}

	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Fatalf("list --json data (-want +got):\n%s", diff)
	}
}

// TestListReportsAnUnreachableDaemon is what an Environments() failure becomes: exitDocker and
// docker_unavailable, in both renderings.
func TestListReportsAnUnreachableDaemon(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		fake := answering(t)
		fake.outcomes = []outcome{{err: errors.New("dial unix /nonexistent: connect: no such file or directory")}}

		stderr := &strings.Builder{}

		code := list(nil, io.Discard, stderr, false)

		if code != exitDocker {
			t.Fatalf("list = %d, want exitDocker", code)
		}

		if !strings.Contains(stderr.String(), "Docker is not running") {
			t.Fatalf("list stderr = %q, want it to mention Docker is not running", stderr.String())
		}
	})

	t.Run("--json", func(t *testing.T) {
		fake := answering(t)
		fake.outcomes = []outcome{{err: errors.New("dial unix /nonexistent: connect: no such file or directory")}}

		stderr := &strings.Builder{}

		code := list(nil, io.Discard, stderr, true)

		if code != exitDocker {
			t.Fatalf("list --json = %d, want exitDocker", code)
		}

		var envelope struct {
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
		}

		if envelope.Error.Type != "docker_unavailable" {
			t.Fatalf("list --json error.type = %q, want %q", envelope.Error.Type, "docker_unavailable")
		}
	})
}
