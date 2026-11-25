package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// --progress=jsonl: the steps the long commands already announce, written to stderr as one JSON
// object per line instead of as sentences. The steps are the ones the use cases say through
// Announce and Progress; nothing here keeps a list of its own.
//

// steppingEngine is the fake with the two things the real engine is handed for its long steps,
// built by the command line exactly as engine() builds them: Rebuild and Setup say what the real
// use cases say on the way.
type steppingEngine struct {
	*fakeEngine

	announce func(string)
	progress func(string) func(bool, string)
}

func (s steppingEngine) Rebuild(dir string, services []string) error {
	s.announce("Rebuilding and starting containers in detached mode\n\n")

	return s.fakeEngine.Rebuild(dir, services)
}

func (s steppingEngine) Setup(dir string, options core.SetupOptions, interactive bool) error {
	s.announce("Starting containers in detached mode\n\n")

	done := s.progress("Importing the database...")
	done(true, "")

	return s.fakeEngine.Setup(dir, options, interactive)
}

func stepping(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test"}

	// The flag sets these for the rest of the process, as --yes and --force do: put back after
	t.Setenv("HM_PROGRESS", "")
	t.Setenv("NO_COLOR", "1")

	newEngine = func(stdout, stderr io.Writer, jsonOutput bool) commands {
		announce, progress := reporting(stdout, stderr, jsonOutput)

		return steppingEngine{fakeEngine: fake, announce: announce, progress: progress}
	}

	return fake
}

type decodedEvent struct {
	Phase   string   `json:"phase"`
	Step    string   `json:"step"`
	Pct     *float64 `json:"pct"`
	Elapsed *float64 `json:"elapsed"`
}

// events decodes stderr line by line, failing on any line that is not one event with exactly the
// four keys.
func events(t *testing.T, stderr string) []decodedEvent {
	t.Helper()

	var decoded []decodedEvent

	lines := bufio.NewScanner(strings.NewReader(stderr))
	for lines.Scan() {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(lines.Bytes(), &keys); err != nil {
			t.Fatalf("stderr line %q is not a JSON object: %v", lines.Text(), err)
		}

		if len(keys) != 4 {
			t.Fatalf("stderr line %s has %d keys, want phase, step, pct and elapsed", lines.Text(), len(keys))
		}

		var event decodedEvent
		if err := json.Unmarshal(lines.Bytes(), &event); err != nil {
			t.Fatalf("stderr line %q: %v", lines.Text(), err)
		}

		if event.Elapsed == nil || *event.Elapsed < 0 {
			t.Fatalf("stderr line %s: elapsed is not a non-negative number", lines.Text())
		}

		if string(keys["pct"]) != "null" {
			t.Fatalf("stderr line %s: pct = %s, want null: no step here knows how far it is", lines.Text(), keys["pct"])
		}

		decoded = append(decoded, event)
	}

	return decoded
}

func TestProgressJSONLWritesTheStepsAsEventsOnStderr(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []decodedEvent
	}{
		{
			"rebuild, the flag before the command",
			[]string{"--progress=jsonl", "rebuild", "--json"},
			[]decodedEvent{{Phase: "rebuild", Step: "Rebuilding and starting containers in detached mode"}},
		},
		{
			"setup, the flag after the command, a step and a timed one",
			[]string{"setup", "--progress=jsonl", "--json"},
			[]decodedEvent{
				{Phase: "setup", Step: "Starting containers in detached mode"},
				{Phase: "setup", Step: "Importing the database..."},
				{Phase: "setup", Step: "Importing the database... done"},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stepping(t)

			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != exitOK {
				t.Fatalf("hm %v = %d, want exitOK\nstderr: %s", tt.args, code, stderr.String())
			}

			got := events(t, stderr.String())
			for at := range got {
				got[at].Elapsed = nil
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("hm %v events (-want +got):\n%s", tt.args, diff)
			}

			// stdout is still the one envelope, and the flag is not an argument of the command
			if count := strings.Count(stdout.String(), "\"schema_version\""); count != 1 {
				t.Fatalf("stdout holds %d envelopes, want 1:\n%s", count, stdout.String())
			}
		})
	}
}

// TestProgressJSONLInTextModeMovesTheStepsOffStdout: in text mode the steps are lines on stdout;
// asked for as events they are events on stderr, and stdout keeps only what is not a step.
func TestProgressJSONLInTextModeMovesTheStepsOffStdout(t *testing.T) {
	stepping(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--progress=jsonl", "--no-json", "rebuild"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("hm rebuild = %d, want exitOK\nstderr: %s", code, stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want the step moved to stderr", stdout.String())
	}

	if got := events(t, stderr.String()); len(got) != 1 || got[0].Step != "Rebuilding and starting containers in detached mode" {
		t.Fatalf("events = %+v, want the rebuild step", got)
	}
}

// Without the flag nothing changes: the step is a sentence on stdout in text mode and on stderr
// under --json, as it was.
func TestWithoutProgressJSONLTheStepsAreSentencesAsBefore(t *testing.T) {
	cases := []struct {
		name               string
		args               []string
		wantOut, wantError string
	}{
		{"text", []string{"--no-json", "rebuild"}, "Rebuilding and starting containers in detached mode\n\n", ""},
		{"json", []string{"--json", "rebuild"}, "", "Rebuilding and starting containers in detached mode\n\n"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stepping(t)

			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != exitOK {
				t.Fatalf("hm %v = %d, want exitOK\nstderr: %s", tt.args, code, stderr.String())
			}

			if stderr.String() != tt.wantError {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantError)
			}

			if tt.wantOut != "" && stdout.String() != tt.wantOut {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantOut)
			}
		})
	}
}

func TestTheProgressPhaseIsTheCommandAndItsSubcommand(t *testing.T) {
	cases := map[string][]string{
		"setup":        {"--progress=jsonl", "setup", "-p=shop"},
		"db snapshot":  {"--json", "db", "snapshot", "--name=x"},
		"db":           {"db", "--name=x"},
		"worktree add": {"worktree", "add", "feature"},
		"rebuild":      {"rebuild", "phpfpm"},
	}

	for want, args := range cases {
		t.Setenv("HM_PROGRESS", "")

		if _, rest := globals(args); progressPhase(rest) != want {
			t.Errorf("progressPhase(%v) = %q, want %q", rest, progressPhase(rest), want)
		}
	}
}

// An unknown value is not swallowed: it reaches the command, which refuses it as an option it does
// not know, rather than a typo silently meaning "no events".
func TestAnUnknownProgressValueIsAUsageError(t *testing.T) {
	stepping(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"rebuild", "--progress=yaml", "--json"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("hm rebuild --progress=yaml = %d, want exitUsage\nstderr: %s", code, stderr.String())
	}
}
