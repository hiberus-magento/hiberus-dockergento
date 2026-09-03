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
		t.Fatalf("bajar desde la última vuelve a la primera, no a %d", at)
	}

	if at := move(0, -1, 3); at != 2 {
		t.Fatalf("subir desde la primera vuelve a la última, no a %d", at)
	}

	if at := move(0, 1, 3); at != 1 {
		t.Fatalf("y en medio se mueve una: %d", at)
	}
}

// A list with nothing in it is not a crash. It is a question nobody can answer, and the caller
// decides what that means.
func TestMovingThroughNothing(t *testing.T) {
	if at := move(0, 1, 0); at != 0 {
		t.Fatalf("una lista vacía no se mueve: %d", at)
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
		t.Fatalf("una pregunta sin respuestas no se puede responder: %v", err)
	}
}

// A choice between options has no safe default, so a run that cannot be asked is refused rather
// than guessed at.
func TestANonInteractiveRunIsRefusedRatherThanGuessed(t *testing.T) {
	t.Setenv("HM_NON_INTERACTIVE", "1")

	_, err := choose("What should happen?", []string{"a", "b"})
	if err == nil {
		t.Fatal("no se puede elegir sin nadie a quien preguntar")
	}

	if err == errNothingChosen {
		t.Fatal("y decirlo es distinto de que no se haya elegido nada")
	}
}

// Aborting a question is not a failure of the command and not an answer either. Said the way a
// shell says it, with the code a shell uses, and without a document: there is nothing to report.
func TestAnAbortedQuestionIsNotAFailure(t *testing.T) {
	stderr := &strings.Builder{}

	if code := report(stderr, true, "down", errNothingChosen); code != exitInterrupted {
		t.Fatalf("abortar una pregunta sale con 130, no con %d", code)
	}

	if !strings.Contains(stderr.String(), "Nothing was chosen") {
		t.Fatalf("y se dice: %q", stderr.String())
	}

	if strings.Contains(stderr.String(), "{") {
		t.Fatalf("sin documento, ni en modo JSON: %q", stderr.String())
	}
}
