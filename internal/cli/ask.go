package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

//
// Asking, when the answer cannot be worked out.
//
// It lives here and not in the engine for the reason everything else about terminals does: an
// HTTP adapter has nobody to ask and would answer some other way. What the engine says is "I need
// this value and here is what I would guess"; who answers is not its business.
//

// ask reads an answer from the terminal, falling back to the suggestion when there is nobody to
// ask.
//
// A question with no suggestion cannot be guessed, so it fails with something actionable instead
// of hanging — which is what a script or an agent needs, and what the shell implementation does.
func ask(text, suggestion string) (string, error) {
	//
	// Only non-interactive mode skips the question. Not "there is no terminal": an answer piped
	// in is still an answer, which is what the shell implementation reads and what lets
	// `printf 'n\n' | hm masquerade` mean no.
	//
	if os.Getenv("HM_NON_INTERACTIVE") != "" {
		if suggestion != "" {
			return suggestion, nil
		}

		return "", fmt.Errorf("non-interactive mode cannot answer: %s", text)
	}

	//
	// The question goes to the error stream, not to stdout: when nobody is watching, stdout
	// carries the document this command answers with, and a question in front of it is something
	// the reader has to parse around. It is also where the shell implementation put it, through
	// `read -p`. A person sees it either way.
	//
	fmt.Fprint(os.Stderr, prompt(text+" ", suggestion))

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(answer) == "" {
		return suggestion, nil //nolint:nilerr
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		return suggestion, nil
	}

	return answer, nil
}
