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
// What logs asks of the engine: its option table, the directory it asks about, and its refusals.
//
// RED-window safety, checked rather than assumed — the one command genuinely safe by
// construction. projectOr (engine.go:77-92) is already routed through newEngine, so the fake
// answers Resolve regardless of this file's own routing state. The unrouted half is only the real
// Logs(cwd, …) call: Resolve(cwd) succeeds (nameless project), orchestrator.open builds a docker
// CLI and Initialize's it without dialling, then load (composelib/orchestrator.go:204-215) fails
// to find a compose file at the temp cwd. No daemon call happens on this path at all — DOCKER_HOST
// is belt-and-braces here, and t.Chdir(t.TempDir()) (inside answering(t)) is what actually bounds
// it. This is the opposite of list and describe, which both need DOCKER_HOST to survive their own
// RED window.
//
// Fixture, load-bearing on two counts. Every test below that reaches the engine sets
// fake.project = core.Project{Name: "shop", Root: "/code/shop"} (inside_test.go:24's value
// verbatim):
//   - Name must be non-empty, or projectOr (engine.go:85) refuses before Logs is ever called — a
//     fixture with only Root set would die at exitProject with the log holding Resolve alone.
//   - Root must be a path that can never equal a t.TempDir() cwd — this is what makes the Dir
//     assertion on the Logs call able to discriminate; leaving it empty would let the assertion
//     pass without proving anything.
//
// checkServices stays off the seam (design Decision 4). A row that names a service still reaches
// the real engine(nil, nil, jsonOutput).Configuration(project) inside checkServices
// (orchestrate.go:198-203). With f.project.Root set to /code/shop, the compose loader finds
// nothing there, returns an error, and :201-203 treats any error as "skip validation" —
// deterministic only while no test in this file creates a compose file under f.project.Root, and
// since that root does not exist on any machine, nothing here can. unknown_service is therefore
// not a planned case in this file: it stays covered by
// tests/integration/logs_launch_version_test.sh and tests/integration/go_orchestration_test.sh,
// both unmodified.
//

func TestWhatLogsAsks(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want func(cwd string) []call
	}{
		{
			name: "no arguments",
			args: nil,
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{}},
				}
			},
		},
		{
			name: "-f/--follow, -t/--timestamps, --no-color and --no-log-prefix all map onto LogOptions",
			args: []string{"-f", "-t", "--no-color", "--no-log-prefix"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{
						Follow: true, Timestamps: true, NoColor: true, NoPrefix: true,
					}},
				}
			},
		},
		{
			name: "--tail maps to LogOptions.Tail",
			args: []string{"--tail", "100"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{Tail: "100"}},
				}
			},
		},
		{
			// -n is --tail's short form (orchestrate.go:111-112): the want is byte-for-byte the
			// same as the row above, on purpose.
			name: "-n is --tail's short form",
			args: []string{"-n", "100"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{Tail: "100"}},
				}
			},
		},
		{
			name: "--since and --until",
			args: []string{"--since", "1h", "--until", "5m"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{Since: "1h", Until: "5m"}},
				}
			},
		},
		{
			name: "--index parses to a number",
			args: []string{"--index", "2"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{Index: 2}},
				}
			},
		},
		{
			name: "positional arguments are services, order preserved",
			args: []string{"phpfpm", "nginx"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{"phpfpm", "nginx"}, LogOptions: core.LogOptions{}},
				}
			},
		},
		{
			// Every flag at once, interleaved with the two service names, proves flag order does
			// not matter and that services and options are collected independently.
			name: "everything at once, flag order does not matter",
			args: []string{
				"phpfpm", "-f", "--tail", "50", "-t", "nginx", "--index", "1",
				"--no-color", "--since", "1h", "--until", "5m", "--no-log-prefix",
			},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					// Dir is the test's working directory, not f.project.Root: orchestrate.go:166
					// asks about here() (os.Getwd()) and never about the project projectOr just
					// resolved. inside_test.go asserts Dir: root for the opposite reason —
					// inside() passes project.Root — so this is where the two precedents diverge.
					// If this line ever fails, the want is right and the change to
					// orchestrate.go is the defect.
					{Method: "Logs", Dir: cwd, Services: []string{"phpfpm", "nginx"}, LogOptions: core.LogOptions{
						Follow: true, Timestamps: true, Tail: "50", Since: "1h", Until: "5m",
						Index: 1, NoColor: true, NoPrefix: true,
					}},
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			code := logs(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("logs %v = %d, want exitOK", tt.args, code)
			}

			if diff := cmp.Diff(tt.want(cwd), fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestLogsRefusesAFlagWithNoValue is takesValue's own guard (orchestrate.go:96): every option
// that expects a value refuses cleanly when it is the last argument, rather than reading past the
// end of args.
func TestLogsRefusesAFlagWithNoValue(t *testing.T) {
	cases := []struct{ argument string }{{"--tail"}, {"-n"}, {"--since"}, {"--until"}, {"--index"}}

	for _, tt := range cases {
		t.Run(tt.argument, func(t *testing.T) {
			fake := answering(t)

			stderr := &strings.Builder{}

			code := logs([]string{tt.argument}, io.Discard, stderr, true)

			if code != exitUsage {
				t.Fatalf("logs %s = %d, want exitUsage", tt.argument, code)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("logs %s asked the engine %v, want nothing asked at all", tt.argument, fake.calls)
			}

			var envelope struct {
				Error struct {
					Type string `json:"type"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Type != "missing_value" {
				t.Fatalf("logs %s error.type = %q, want missing_value", tt.argument, envelope.Error.Type)
			}
		})
	}
}

// TestLogsRefusesANonNumericIndex is orchestrate.go:117-122's own guard: --index only accepts
// what strconv.Atoi accepts.
func TestLogsRefusesANonNumericIndex(t *testing.T) {
	fake := answering(t)

	stderr := &strings.Builder{}

	code := logs([]string{"--index", "nada"}, io.Discard, stderr, true)

	if code != exitUsage {
		t.Fatalf("logs --index nada = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("logs --index nada asked the engine %v, want nothing asked at all", fake.calls)
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
		t.Fatalf("logs --index nada error.type = %q, want invalid_argument", envelope.Error.Type)
	}
}

// TestLogsRefusesAnUnknownOption pins orchestrate.go:140-143's default branch, the same shape as
// TestASetHostOptionNobodyDeclaredIsAUsageError.
func TestLogsRefusesAnUnknownOption(t *testing.T) {
	fake := answering(t)

	code := logs([]string{"--tonteria"}, io.Discard, io.Discard, false)

	if code != exitUsage {
		t.Fatalf("logs --tonteria = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("logs --tonteria asked the engine %v, want nothing asked at all", fake.calls)
	}
}

// TestLogsReportsAFailure is what a failed Logs call becomes: exitDocker and docker_failed, the
// generic report() shape every engine refusal that is not a core.Refusal already gets
// (docker_stop_all_test.go's TestAFailedStopIsReportedAsADockerFailure is the same shape). The
// outcome that fails is outcomes[1] — the Logs call — not outcomes[0], which is Resolve and must
// succeed first for Logs to be reached at all.
func TestLogsReportsAFailure(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: errors.New("could not read logs")}}
	cwd := here()

	stderr := &strings.Builder{}

	code := logs(nil, io.Discard, stderr, true)

	if code != exitDocker {
		t.Fatalf("logs with a failure = %d, want exitDocker", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		// Dir is the test's working directory, not f.project.Root: orchestrate.go:166 asks about
		// here() (os.Getwd()) and never about the project projectOr just resolved. inside_test.go
		// asserts Dir: root for the opposite reason — inside() passes project.Root — so this is
		// where the two precedents diverge. If this line ever fails, the want is right and the
		// change to orchestrate.go is the defect.
		{Method: "Logs", Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{}},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
	}

	if envelope.Error.Type != "docker_failed" {
		t.Fatalf("logs error.type = %q, want docker_failed", envelope.Error.Type)
	}
}
