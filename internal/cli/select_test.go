package cli

import (
	"strings"
	"testing"
)

//
// The two halves of the list that can be tested without a terminal: where the selection goes, and
// what the list looks like.
//

func TestMovingWrapsAtBothEnds(t *testing.T) {
	// Somebody at the last option pressing down means the first one, not a beep
	if at := move(2, 1, 3); at != 0 {
		t.Fatalf("move(last, down) = %d, want 0", at)
	}

	if at := move(0, -1, 3); at != 2 {
		t.Fatalf("move(first, up) = %d, want the last", at)
	}

	if at := move(0, 1, 3); at != 1 {
		t.Fatalf("move(0, down) = %d, want 1", at)
	}
}

// A list with nothing in it is not a crash. It is a question nobody can answer, and the caller
// decides what that means.
func TestMovingThroughNothing(t *testing.T) {
	if at := move(0, 1, 0); at != 0 {
		t.Fatalf("move(0, down, nothing) = %d, want 0", at)
	}
}

// The marker is the first thing on the line rather than a colour, because a colour is what a
// monochrome terminal swallows — and what a test comparing text would not see either.
func TestTheChosenOneIsMarked(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	drawn := renderOptions(1, []string{"guardar", "destruir", "cancelar"})

	want := "    1) guardar\n  ❯ 2) destruir\n    3) cancelar\n"

	if drawn != want {
		t.Fatalf("la lista se dibuja distinta:\n%q\nen vez de\n%q", drawn, want)
	}
}

// Nothing chosen is not an empty answer: a caller that got one would carry on with nothing
// chosen, and for a destructive question that is the wrong branch.
func TestChoosingFromNothing(t *testing.T) {
	if _, err := choose("¿qué?", nil); err != errNothingChosen {
		t.Fatalf("choose(no options) = %v, want errNothingChosen", err)
	}
}

// A choice between options has no safe default, so a run that cannot be asked is refused rather
// than guessed at.
func TestANonInteractiveRunIsRefusedRatherThanGuessed(t *testing.T) {
	t.Setenv("HM_NON_INTERACTIVE", "1")

	_, err := choose("What should happen?", []string{"a", "b"})
	if err == nil {
		t.Fatal("choose in non-interactive mode = nil, want a refusal")
	}

	if err == errNothingChosen {
		t.Fatal("choose in non-interactive mode = errNothingChosen, want a refusal that says why")
	}
}

// Aborting a question is not a failure of the command and not an answer either. Said the way a
// shell says it, with the code a shell uses, and without a document: there is nothing to report.
func TestAnAbortedQuestionIsNotAFailure(t *testing.T) {
	stderr := &strings.Builder{}

	if code := report(stderr, true, "down", errNothingChosen); code != exitInterrupted {
		t.Fatalf("report(errNothingChosen) = %d, want 130", code)
	}

	if !strings.Contains(stderr.String(), "Nothing was chosen") {
		t.Fatalf("stderr = %q, want it to say nothing was chosen", stderr.String())
	}

	if strings.Contains(stderr.String(), "{") {
		t.Fatalf("stderr = %q, want no document, not even in JSON mode", stderr.String())
	}
}

//
// A terminal that cannot be drawn on still gets a list it can answer.
//

func TestAPipeCannotBeDrawnOn(t *testing.T) {
	// TERM says it could draw, so what refuses is the pipe at both ends
	t.Setenv("TERM", "xterm-256color")

	var drawable bool

	askingWith(t, "", func() { drawable = canDraw() })

	if drawable {
		t.Fatal("canDraw() with stdin and stdout on pipes = true, want false")
	}
}

func TestTheNumberedListStillChoosesByNumber(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("HM_NON_INTERACTIVE", "")
	t.Setenv("TERM", "xterm-256color")

	var chosen string
	var err error

	// Through choose, so it is the fallback a non-drawable input actually gets that is checked
	stdout, _ := askingWith(t, "2\n", func() {
		chosen, err = choose("¿qué hacemos?", []string{"guardar", "destruir", "cancelar"})
	})

	if err != nil || chosen != "destruir" {
		t.Fatalf("choose with \"2\" piped in = (%q, %v), want (destruir, no error)", chosen, err)
	}

	if !strings.Contains(stdout, "2) destruir") {
		t.Fatalf("stdout = %q, want the numbered list drawn", stdout)
	}
}
