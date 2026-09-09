package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What down refuses before it ever reaches an engine. No answering(t), no fake engine, no Docker:
// every refusal here returns during parsing, before projectOr (engine.go:77-92, called from
// down.go:55) is ever reached — except TestDownStillAcceptsAGivenValue, which deliberately reaches
// it. This diverges from version_test.go and set_host_test.go, which use answering(t)'s
// fake+chdir+env fixture: this file needs only the t.Chdir(t.TempDir()) half of that fixture,
// applied per test, never the fake.
//

func TestDownRefusesAMissingRmiValue(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		t.Chdir(t.TempDir())

		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := down([]string{"--rmi"}, stdout, stderr, false)

		if code != exitUsage {
			t.Fatalf("down --rmi = %d, want exitUsage", code)
		}

		if stdout.String() != "" {
			t.Fatalf("down --rmi stdout = %q, want empty", stdout.String())
		}

		if !strings.Contains(stderr.String(), "--rmi needs a value") {
			t.Fatalf("down --rmi stderr = %q, want it to name the flag", stderr.String())
		}

		if !strings.Contains(stderr.String(), "  → hm down --rmi all") {
			t.Fatalf("down --rmi stderr = %q, want the hint", stderr.String())
		}
	})

	t.Run("--json", func(t *testing.T) {
		t.Chdir(t.TempDir())

		stdout, stderr := &strings.Builder{}, &strings.Builder{}

		code := down([]string{"--rmi"}, stdout, stderr, true)

		if code != exitUsage {
			t.Fatalf("down --rmi --json = %d, want exitUsage", code)
		}

		if stdout.String() != "" {
			t.Fatalf("down --rmi --json stdout = %q, want empty", stdout.String())
		}

		var envelope struct {
			Error struct {
				Code    int    `json:"code"`
				Type    string `json:"type"`
				Message string `json:"message"`
				Hint    string `json:"hint"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
		}

		want := struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		}{Code: 2, Type: "missing_value", Message: "--rmi needs a value", Hint: "hm down --rmi all"}

		if diff := cmp.Diff(want, envelope.Error); diff != "" {
			t.Fatalf("down --rmi --json refusal (-want +got):\n%s", diff)
		}
	})
}

func TestDownRefusesAMissingTimeoutValue(t *testing.T) {
	cases := []struct{ argument string }{{"-t"}, {"--timeout"}}

	for _, tt := range cases {
		t.Run(tt.argument, func(t *testing.T) {
			t.Run("text", func(t *testing.T) {
				t.Chdir(t.TempDir())

				stdout, stderr := &strings.Builder{}, &strings.Builder{}

				code := down([]string{tt.argument}, stdout, stderr, false)

				if code != exitUsage {
					t.Fatalf("down %s = %d, want exitUsage", tt.argument, code)
				}

				if stdout.String() != "" {
					t.Fatalf("down %s stdout = %q, want empty", tt.argument, stdout.String())
				}

				if !strings.Contains(stderr.String(), tt.argument+" needs a value") {
					t.Fatalf("down %s stderr = %q, want it to name the flag", tt.argument, stderr.String())
				}

				if !strings.Contains(stderr.String(), "  → hm down -t 30") {
					t.Fatalf("down %s stderr = %q, want the hint", tt.argument, stderr.String())
				}
			})

			t.Run("--json", func(t *testing.T) {
				t.Chdir(t.TempDir())

				stdout, stderr := &strings.Builder{}, &strings.Builder{}

				code := down([]string{tt.argument}, stdout, stderr, true)

				if code != exitUsage {
					t.Fatalf("down %s --json = %d, want exitUsage", tt.argument, code)
				}

				if stdout.String() != "" {
					t.Fatalf("down %s --json stdout = %q, want empty", tt.argument, stdout.String())
				}

				var envelope struct {
					Error struct {
						Code    int    `json:"code"`
						Type    string `json:"type"`
						Message string `json:"message"`
						Hint    string `json:"hint"`
					} `json:"error"`
				}

				if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
					t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
				}

				want := struct {
					Code    int    `json:"code"`
					Type    string `json:"type"`
					Message string `json:"message"`
					Hint    string `json:"hint"`
				}{Code: 2, Type: "missing_value", Message: tt.argument + " needs a value", Hint: "hm down -t 30"}

				if diff := cmp.Diff(want, envelope.Error); diff != "" {
					t.Fatalf("down %s --json refusal (-want +got):\n%s", tt.argument, diff)
				}
			})
		})
	}
}

// TestDownStillRefusesANonNumericTimeout pins the existing invalid_argument behavior (down.go:42-44)
// so the new missing-value guard cannot regress it. Run against unmodified down.go this already
// passes; it is not a RED test, it is a safety net for the guard about to be added above it.
func TestDownStillRefusesANonNumericTimeout(t *testing.T) {
	t.Chdir(t.TempDir())

	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	code := down([]string{"-t", "nada"}, stdout, stderr, true)

	if code != exitUsage {
		t.Fatalf("down -t nada = %d, want exitUsage", code)
	}

	if stdout.String() != "" {
		t.Fatalf("down -t nada stdout = %q, want empty", stdout.String())
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
	}

	if envelope.Error.Type != "invalid_argument" {
		t.Fatalf("down -t nada error.type = %q, want invalid_argument — distinguishable from missing_value", envelope.Error.Type)
	}
}

// TestDownStillAcceptsAGivenValue is a safety-pin: from a directory that is not a project,
// projectOr (engine.go:84-89) fails resolution before down.go:61's engine(...).Down(...) — the
// concrete, non-seamed call — is ever reached, so this never touches a real Docker daemon.
//
// TestDownStillAcceptsAGivenValue and TestADownOptionNobodyDeclaredIsAUsageError are also where
// down-go-tests should start: the delta spec's Handoff Note (spec.md:72-79) records that
// archive/2026-09-05-down-and-set-host-go-tests/design.md:306 and :340-342 still plan to pin
// --rmi/-t last as silently-ignored core.DownOptions{}; that plan is stale once this file lands
// refusals for both, and down-go-tests extends this file rather than authoring it from scratch.
func TestDownStillAcceptsAGivenValue(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"--rmi all", []string{"--rmi", "all"}},
		{"-t 30", []string{"-t", "30"}},
		{"-v --remove-orphans --rmi all -t 30", []string{"-v", "--remove-orphans", "--rmi", "all", "-t", "30"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := down(tt.args, stdout, stderr, true)

			// This does not prove --rmi all stores "all" in core.DownOptions.Images, nor that -t 30
			// stores 30 — Down is off the commands seam (engine.go:17-30) until down-go-tests grows
			// it (archive/2026-09-05-down-and-set-host-go-tests/design.md:291-296). Asserting the
			// exact resolution failure is the strongest statement available without it. Stated gap,
			// not an oversight.
			if code != exitProject {
				t.Fatalf("down %v = %d, want exitProject (parsed past both guards, refused only at project resolution)", tt.args, code)
			}

			if stdout.String() != "" {
				t.Fatalf("down %v stdout = %q, want empty", tt.args, stdout.String())
			}

			var envelope struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Type != "project_not_configured" {
				t.Fatalf("down %v error.type = %q, want exactly project_not_configured, never just \"any code other than 2\"", tt.args, envelope.Error.Type)
			}
		})
	}
}

// TestADownOptionNobodyDeclaredIsAUsageError pins the existing default: branch, confirming neither
// new guard touched it.
func TestADownOptionNobodyDeclaredIsAUsageError(t *testing.T) {
	t.Chdir(t.TempDir())

	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	code := down([]string{"--tonteria"}, stdout, stderr, false)

	if code != exitUsage {
		t.Fatalf("down --tonteria = %d, want exitUsage", code)
	}

	if stdout.String() != "" {
		t.Fatalf("down --tonteria stdout = %q, want empty", stdout.String())
	}

	if !strings.Contains(stderr.String(), "Unknown option: --tonteria") {
		t.Fatalf("down --tonteria stderr = %q, want it to name the option", stderr.String())
	}

	if !strings.Contains(stderr.String(), "  → hm down [-v]") {
		t.Fatalf("down --tonteria stderr = %q, want the hint", stderr.String())
	}
}
