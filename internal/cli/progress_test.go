package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

//
// Saying that something is still happening, without drawing where nobody is looking.
//
// A spinner in a log file is noise a reader has to scroll past, and an escape sequence there is
// a difference from the shell implementation's output, byte for byte.
//

// drawableEnvironment clears every variable that asks for quiet and sets a TERM that can draw, so
// that whatever refuses to animate in a test is the thing the test names.
func drawableEnvironment(t *testing.T) {
	t.Helper()

	for _, quiet := range []string{
		"NO_COLOR", "HM_NO_COLOR", "HM_NON_INTERACTIVE", "HM_NO_PROGRESS",
		"FORCE_COLOR", "CLICOLOR_FORCE",
	} {
		t.Setenv(quiet, "")
	}

	t.Setenv("TERM", "xterm-256color")
}

func TestAPipeIsNeverAnimated(t *testing.T) {
	drawableEnvironment(t)

	var animated, animatedForJSON bool

	// stdout is a pipe for the length of the call: everything else would draw
	askingWith(t, "", func() {
		animated = animates(false)
		animatedForJSON = animates(true)
	})

	if animated || animatedForJSON {
		t.Fatalf("animates(text, json) on a pipe = (%v, %v), want (false, false)", animated, animatedForJSON)
	}
}

func TestAStepSaysWhatIsAboutToHappenWithoutEscapeSequences(t *testing.T) {
	drawableEnvironment(t)

	var written bytes.Buffer

	askingWith(t, "", func() {
		done := begin("Importing the database...", &written, &written, false)
		done(true, "")
	})

	if !strings.HasPrefix(written.String(), "Importing the database...\n") {
		t.Fatalf("output = %q, want the step announced before it runs", written.String())
	}

	if !strings.Contains(written.String(), "Importing the database... done\n") {
		t.Fatalf("output = %q, want the step reported done", written.String())
	}

	if strings.Contains(written.String(), "\033") {
		t.Fatalf("output = %q, want no escape sequences on a non-terminal", written.String())
	}
}

func TestDurationsAreSecondsUnderAMinuteAndMinutesBeyond(t *testing.T) {
	cases := map[int]string{
		2:   "2s",
		59:  "59s",
		60:  "1m00s",
		125: "2m05s",
	}

	for seconds, want := range cases {
		if got := duration(seconds); got != want {
			t.Errorf("duration(%d) = %q, want %q", seconds, got, want)
		}
	}
}

func TestUnderTwoSecondsNoStopwatchIsShown(t *testing.T) {
	// Everything takes a moment, so a number under two seconds is noise
	t.Setenv("NO_COLOR", "1")

	var quick, slow bytes.Buffer

	finish(&quick, "Starting", "", time.Now(), true)
	finish(&slow, "Starting", "", time.Now().Add(-3*time.Second), true)

	if quick.String() != "Starting done\n" {
		t.Fatalf("a quick step = %q, want no elapsed time", quick.String())
	}

	if slow.String() != "Starting done (3s)\n" {
		t.Fatalf("a slow step = %q, want its elapsed time", slow.String())
	}
}
