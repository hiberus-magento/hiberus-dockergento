package cli

import (
	"os"
	"strings"
	"testing"
)

//
// Where a question is written.
//
// A question is for a person, and the answer a command gives is not always for one: when the
// output is not a terminal the tool answers a document, and anything else on that stream is
// something the reader has to parse around. So the question goes where the rest of the decoration
// goes — the error stream — which is also where the shell implementation put it, through
// `read -p`.
//
// choose() has no such problem: it refuses to draw at all when stdout is not a terminal
// (select.go:59). ask() cannot refuse, because an answer piped in is still an answer, so it has
// to write somewhere that is not the document.
//

// answering swaps the three standard streams for pipes and hands back what was written to each,
// once the function under test has returned.
func askingWith(t *testing.T, answer string, run func()) (stdout, stderr string) {
	t.Helper()

	realIn, realOut, realErr := os.Stdin, os.Stdout, os.Stderr

	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() = %v, want no error", err)
	}

	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() = %v, want no error", err)
	}

	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() = %v, want no error", err)
	}

	os.Stdin, os.Stdout, os.Stderr = inRead, outWrite, errWrite

	t.Cleanup(func() { os.Stdin, os.Stdout, os.Stderr = realIn, realOut, realErr })

	go func() {
		_, _ = inWrite.WriteString(answer)
		_ = inWrite.Close()
	}()

	run()

	_ = outWrite.Close()
	_ = errWrite.Close()

	return read(t, outRead), read(t, errRead)
}

func read(t *testing.T, from *os.File) string {
	t.Helper()

	var builder strings.Builder

	buffer := make([]byte, 4096)

	for {
		n, err := from.Read(buffer)
		builder.Write(buffer[:n])

		if err != nil {
			break
		}
	}

	return builder.String()
}

// A question asked while the answer is being captured must not land in the answer.
func TestAQuestionIsWrittenWhereTheDocumentIsNot(t *testing.T) {
	t.Setenv("HM_NON_INTERACTIVE", "")

	var answer string

	stdout, stderr := askingWith(t, "n\n", func() {
		got, err := ask("Stop them all? [y/N]:", "")
		if err != nil {
			t.Errorf("ask() = %v, want no error", err)
		}

		answer = got
	})

	if answer != "n" {
		t.Errorf("answer = %q, want the one that was piped in", answer)
	}

	if !strings.Contains(stderr, "Stop them all?") {
		t.Errorf("stderr = %q, want the question", stderr)
	}

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing: it is where the document goes", stdout)
	}
}

// Nobody to ask is unchanged by where the question would have been written.
func TestWithNobodyToAskTheSuggestionAnswers(t *testing.T) {
	t.Setenv("HM_NON_INTERACTIVE", "1")

	got, err := ask("Stop them all? [y/N]:", "N")
	if err != nil {
		t.Fatalf("ask() = %v, want no error", err)
	}

	if got != "N" {
		t.Errorf("answer = %q, want the suggestion", got)
	}
}
