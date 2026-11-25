package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"nothing", 0, "0 B"},
		{"under a kilobyte", 999, "999 B"},
		{"one kilobyte", 1000, "1.0 kB"},
		{"rounds up into the next whole number", 9960, "10 kB"},
		{"rounds up into the next unit", 999_600, "1.0 MB"},
		{"a vendor", 412_000_000, "412 MB"},
		{"a large one", 1_500_000_000, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanBytes(tt.bytes); got != tt.want {
				t.Fatalf("humanBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatCopy(t *testing.T) {
	tests := []struct {
		name  string
		state copyState
		want  string
	}{
		{
			name:  "known total, under way",
			state: copyState{bytes: 412_000_000, files: 38120, total: 675_000_000, elapsed: 8 * time.Second},
			want:  "  ████████████░░░░░░░░  61%  412 MB / 675 MB · 38120 files · 52 MB/s · ~6s",
		},
		{
			name:  "known total, nothing carried yet",
			state: copyState{total: 675_000_000},
			want:  "  ░░░░░░░░░░░░░░░░░░░░   0%  0 B / 675 MB · 0 files",
		},
		{
			name:  "known total, finished: the total becomes what was carried",
			state: copyState{bytes: 650_000_000, files: 40000, total: 675_000_000, elapsed: 13 * time.Second, done: true},
			want:  "  ████████████████████ 100%  650 MB / 650 MB · 40000 files · 50 MB/s · 13s",
		},
		{
			name:  "more carried than measured never passes a hundred",
			state: copyState{bytes: 700_000_000, files: 2, total: 675_000_000, elapsed: 10 * time.Second},
			want:  "  ████████████████████ 100%  700 MB / 700 MB · 2 files · 70 MB/s",
		},
		{
			name:  "unknown total, under way: a counter",
			state: copyState{bytes: 412_000_000, files: 38120, elapsed: 8 * time.Second},
			want:  "  412 MB · 38120 files · 52 MB/s",
		},
		{
			name:  "unknown total, finished",
			state: copyState{bytes: 412_000_000, files: 1, elapsed: 75 * time.Second, done: true},
			want:  "  412 MB · 1 file · 5.5 MB/s · 1m15s",
		},
		{
			name:  "zero, at once",
			state: copyState{done: true},
			want:  "  0 B · 0 files · 0s",
		},
		{
			name:  "a long wait left",
			state: copyState{bytes: 10_000_000, files: 5, total: 1_000_000_000, elapsed: 10 * time.Second},
			want:  "  ░░░░░░░░░░░░░░░░░░░░   1%  10 MB / 1.0 GB · 5 files · 1.0 MB/s · ~16m30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCopy(tt.state, false); got != tt.want {
				t.Fatalf("formatCopy() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestFormatCopyPaintsOnlyTheFilledBarWhenColourIsAllowed(t *testing.T) {
	state := copyState{bytes: 50, files: 1, total: 100, elapsed: time.Second}

	got := formatCopy(state, true)
	want := "  \033[0;32m██████████\033[0m░░░░░░░░░░  50%  50 B / 100 B · 1 file · 50 B/s · ~1s"

	if got != want {
		t.Fatalf("formatCopy(colour) =\n%q\nwant\n%q", got, want)
	}
}

// Nothing but a person at a terminal gets a progress line: the engine is handed no watch at all,
// which is what keeps a log, a pipe and --json byte for byte what they were.
func TestCopyWatchIsOnlyForATerminal(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"CI", "HM_NO_PROGRESS", "HM_NON_INTERACTIVE", "NO_COLOR", "HM_NO_COLOR"} {
				t.Setenv(name, "")
			}

			t.Setenv("TERM", "xterm-256color")

			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			was := copyTerminal
			copyTerminal = func(any) bool { return tt.terminal }

			t.Cleanup(func() { copyTerminal = was })

			if got := copyWatch(&bytes.Buffer{}, tt.json) != nil; got != tt.want {
				t.Fatalf("copyWatch() given = %v, want %v", got, tt.want)
			}
		})
	}
}

// The line is redrawn in place and left finished, on a line of its own, when the copy ends.
func TestCopyLineEndsWithTheFinalStateOnItsOwnLine(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var out bytes.Buffer

	meter, end := drawCopy(&out, 2000, false)
	meter(1000, 1)
	meter(2000, 2)
	end()

	got := out.String()

	if !strings.HasPrefix(got, "\r") || !strings.HasSuffix(got, "\n") {
		t.Fatalf("output %q, want a carriage return before the line and a newline after it", got)
	}

	final := got[strings.LastIndex(got, "\r")+1:]
	if !strings.Contains(final, "100%  2.0 kB / 2.0 kB · 2 files") {
		t.Fatalf("final state %q, want the whole copy", final)
	}

	if strings.Contains(got, "\033") {
		t.Fatalf("output %q carries an escape sequence without colour", got)
	}
}

// A shorter line drawn over a longer one is padded, so nothing of the old one is left behind.
func TestCopyLinePadsOverALongerLine(t *testing.T) {
	var out bytes.Buffer

	line := &copyLine{out: &out}
	line.draw("  0123456789")
	line.draw("  01")

	if got, want := out.String(), "\r  0123456789\r  01        "; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}
