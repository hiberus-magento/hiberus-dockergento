package cli

import (
	"bytes"
	"strings"
	"testing"
)

//
// What update and switch show on a terminal while they move the tool to another release. Which
// steps the engine takes, and in which order, is in dockergento/release_test.go; here, how the
// command line draws them.
//

// onATerminal makes stderr a terminal that can draw, in UTF-8, with nothing asking for silence, for
// one test; colour is off unless asked for.
func onATerminal(t *testing.T, colour bool) {
	t.Helper()

	for _, name := range []string{"CI", "HM_NO_PROGRESS", "HM_NON_INTERACTIVE", "NO_COLOR", "HM_NO_COLOR", "HM_PROGRESS", "LC_ALL", "LC_CTYPE"} {
		t.Setenv(name, "")
	}

	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LANG", "en_US.UTF-8")

	if !colour {
		t.Setenv("NO_COLOR", "1")
	}

	was := copyTerminal
	copyTerminal = func(any) bool { return true }

	t.Cleanup(func() { copyTerminal = was })
}

// screen is what a terminal is left showing after out: a carriage return goes back to the start of
// the line and writes over it, a newline ends it. Only for output without colour sequences.
func screen(out string) []string {
	lines := []string{}
	current := []rune{}
	column := 0

	for _, r := range out {
		switch r {
		case '\r':
			column = 0
		case '\n':
			lines = append(lines, strings.TrimRight(string(current), " "))
			current, column = current[:0], 0
		default:
			if column < len(current) {
				current[column] = r
			} else {
				current = append(current, r)
			}
			column++
		}
	}

	if last := strings.TrimRight(string(current), " "); last != "" {
		lines = append(lines, last)
	}

	return lines
}

// Nothing but a person at a terminal is shown the steps, under the same rules as the copy line: the
// engine is handed no watch at all, so a log, a pipe and --json read what they always read.
func TestReleaseWatchIsOnlyForATerminal(t *testing.T) {
	tests := []struct {
		name     string
		terminal bool
		json     bool
		env      map[string]string
		want     bool
	}{
		{"a terminal", true, false, nil, true},
		{"redirected", false, false, nil, false},
		{"json on a terminal", true, true, nil, false},
		{"CI", true, false, map[string]string{"CI": "true"}, false},
		{"asked for silence", true, false, map[string]string{"HM_NO_PROGRESS": "1"}, false},
		{"non interactive", true, false, map[string]string{"HM_NON_INTERACTIVE": "1"}, false},
		{"a terminal that cannot draw", true, false, map[string]string{"TERM": "dumb"}, false},
		{"NO_COLOR still draws, without colour", true, false, map[string]string{"NO_COLOR": "1"}, true},
		{"progress as JSON lines on a terminal", true, false, map[string]string{"HM_PROGRESS": "jsonl"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onATerminal(t, true)

			copyTerminal = func(any) bool { return tt.terminal }

			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			watch := releaseWatch(&bytes.Buffer{}, tt.json)

			if got := watch.Step != nil && watch.Download != nil; got != tt.want {
				t.Fatalf("releaseWatch() watches = %v, want %v", got, tt.want)
			}

			if !tt.want && (watch.Step != nil || watch.Download != nil) {
				t.Fatal("half a watch: either both or neither")
			}
		})
	}
}

// An update on a terminal: each step is a spinner while it runs and a tick when it is done, and the
// download is the copy line, without a count of files.
func TestAnUpdateOnATerminalShowsEachStep(t *testing.T) {
	onATerminal(t, false)

	var out bytes.Buffer

	watch := releaseWatch(&out, false)

	end := watch.Step("Looking up the latest release")
	end("Latest pre-release: 2.0.0-rc.5 (you have 2.0.0-rc.4)", false)

	meter, finish := watch.Download(26_000_000)
	meter(13_000_000, 1)
	meter(26_000_000, 1)
	finish(false)

	end = watch.Step("Verifying the checksum")
	end("Checksum verified", false)

	end = watch.Step("Installing 2.0.0-rc.5")
	end("Installed at /usr/local/bin/hm", false)

	raw := out.String()

	if !strings.Contains(raw, "⠋ Looking up the latest release") {
		t.Fatalf("output %q, want the look-up drawn running with a spinner first", raw)
	}

	if strings.Contains(raw, "\033") {
		t.Fatalf("output %q carries an escape sequence without colour", raw)
	}

	shown := screen(raw)
	t.Logf("a terminal shows:\n%s", strings.Join(shown, "\n"))

	if len(shown) != 4 {
		t.Fatalf("a terminal shows %d lines, want 4:\n%s", len(shown), strings.Join(shown, "\n"))
	}

	if shown[0] != "✓ Latest pre-release: 2.0.0-rc.5 (you have 2.0.0-rc.4)" {
		t.Errorf("line 1 = %q", shown[0])
	}

	if !strings.HasPrefix(shown[1], "  ████████████████████ 100%  26 MB / 26 MB · ") || strings.Contains(shown[1], "file") {
		t.Errorf("line 2 = %q, want the whole download, with no count of files", shown[1])
	}

	if shown[2] != "✓ Checksum verified" || shown[3] != "✓ Installed at /usr/local/bin/hm" {
		t.Errorf("lines 3 and 4 = %q, %q", shown[2], shown[3])
	}
}

// Up to date, the answer is the command's own: the spinner goes and leaves nothing behind.
func TestAStepEndedWithNothingToSayLeavesNothing(t *testing.T) {
	onATerminal(t, false)

	var out bytes.Buffer

	end := releaseWatch(&out, false).Step("Looking up the latest release")
	end("", false)

	if !strings.Contains(out.String(), "Looking up the latest release") {
		t.Fatalf("output %q, want the step drawn while it ran", out.String())
	}

	if shown := screen(out.String()); len(shown) != 0 || strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("a terminal shows %q (output %q), want nothing and the cursor at the start of the line", shown, out.String())
	}
}

// A failed step is a red cross before the error, and the tick of one that worked is green.
func TestStepsAreMarkedInColour(t *testing.T) {
	onATerminal(t, true)

	var out bytes.Buffer

	watch := releaseWatch(&out, false)
	watch.Step("Verifying the checksum")("Checksum verified", false)

	// Padded with spaces over what is left of the longer running line
	if !strings.Contains(out.String(), "\r\033[0;32m✓\033[0m Checksum verified") || !strings.HasSuffix(out.String(), " \n") {
		t.Fatalf("output %q, want a green tick", out.String())
	}

	out.Reset()
	watch.Step("Looking up the latest release")("", true)

	if !strings.HasSuffix(out.String(), "\r\033[0;31m✗\033[0m Looking up the latest release\n") {
		t.Fatalf("output %q, want a red cross on the step's own words", out.String())
	}
}

func TestAFailedStepWithoutColourIsStillACross(t *testing.T) {
	onATerminal(t, false)

	var out bytes.Buffer

	releaseWatch(&out, false).Step("Installing 2.1.0")("", true)

	if shown := screen(out.String()); len(shown) != 1 || shown[0] != "✗ Installing 2.1.0" {
		t.Fatalf("a terminal shows %q", shown)
	}
}

// A server that announces no length gets a counter: no bar, no percentage, no time left.
func TestADownloadOfUnknownLengthIsACounter(t *testing.T) {
	onATerminal(t, false)

	var out bytes.Buffer

	meter, finish := releaseWatch(&out, false).Download(0)
	meter(1500, 1)
	finish(false)

	shown := screen(out.String())
	if len(shown) != 1 || !strings.HasPrefix(shown[0], "  1.5 kB · ") ||
		strings.ContainsAny(shown[0], "%█░") || strings.Contains(shown[0], "file") {
		t.Fatalf("a terminal shows %q, want a counter of bytes", shown)
	}
}

// A download cut short is left where it stopped, and said.
func TestADownloadCutShortIsShownFailed(t *testing.T) {
	onATerminal(t, false)

	var out bytes.Buffer

	meter, finish := releaseWatch(&out, false).Download(2000)
	meter(500, 1)
	finish(true)

	shown := screen(out.String())
	if len(shown) != 1 || !strings.Contains(shown[0], " 25%  500 B / 2.0 kB") || !strings.HasSuffix(shown[0], "failed") {
		t.Fatalf("a terminal shows %q", shown)
	}
}
