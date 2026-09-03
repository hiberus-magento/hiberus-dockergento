package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)

//
// A list you move through.
//
// Three ways of asking, chosen once and for the reasons the shell implementation gives: somebody
// who installed `fzf` has opinions about picking from a list and this tool has no business
// overriding them; everybody else gets the arrow keys; and a terminal that cannot be drawn on
// still gets the numbered list, which has always been good at working anywhere.
//
// The parts that can be tested without a terminal are kept apart from the part that cannot:
// moving through a list and drawing it are two functions of an index and some strings.
//

// errNothingChosen is what aborting the list means. It is not an empty answer: a caller that got
// one would carry on with nothing chosen, and for `down -v` that is the wrong branch of a
// destructive question.
var errNothingChosen = fmt.Errorf("nothing was chosen")

// choose asks somebody to pick one of several answers.
func choose(question string, options []string) (string, error) {
	if len(options) == 0 {
		return "", errNothingChosen
	}

	// A choice between options has no safe default: picking one silently could destroy the thing
	// the question was about
	if os.Getenv("HM_NON_INTERACTIVE") != "" {
		return "", refusal("input_required", exitUsage,
			fmt.Sprintf("Non-interactive mode cannot choose: %s (options: %s)",
				question, strings.Join(options, " ")),
			"Pass the value as an option, or run without --yes")
	}

	if hasFzf() {
		return withFzf(question, options)
	}

	if canDraw() {
		return withArrows(question, options)
	}

	return numbered(question, options)
}

// hasFzf reports whether to hand the question over.
func hasFzf() bool {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return false
	}

	_, err := exec.LookPath("fzf")

	return err == nil
}

// canDraw asks whether this terminal can be drawn on.
//
// Both ends have to be one — the keys are read from stdin and the list is drawn on stdout — and
// TERM has to describe something that understands cursor movement.
func canDraw() bool {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return false
	}

	switch os.Getenv("TERM") {
	case "", "dumb":
		return false
	}

	return true
}

// withFzf hands the question to what the person installed.
func withFzf(question string, options []string) (string, error) {
	command := exec.Command("fzf", "--height=~40%", "--reverse", "--no-multi",
		"--prompt="+question+" ")
	command.Stdin = strings.NewReader(strings.Join(options, "\n") + "\n")
	command.Stderr = os.Stderr

	chosen, err := command.Output()
	if err != nil || strings.TrimSpace(string(chosen)) == "" {
		return "", errNothingChosen
	}

	return strings.TrimSpace(string(chosen)), nil
}

// withArrows is the selector: move with the arrows or with j and k, take with Enter, or press the
// digit of an option.
//
// Escape does nothing, on purpose. See errNothingChosen.
func withArrows(question string, options []string) (string, error) {
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return numbered(question, options)
	}
	defer term.Restore(int(os.Stdin.Fd()), state) //nolint:errcheck

	selected := 0

	fmt.Fprint(os.Stdout, section(question)+"\r\n")
	fmt.Fprint(os.Stdout, strings.ReplaceAll(renderOptions(selected, options), "\n", "\r\n"))
	fmt.Fprint(os.Stdout, "\033[?25l")

	defer fmt.Fprint(os.Stdout, "\033[?25h")

	reader := bufio.NewReader(os.Stdin)

	for {
		key := readKey(reader)

		switch key {
		case "up", "k":
			selected = move(selected, -1, len(options))
		case "down", "j":
			selected = move(selected, 1, len(options))
		case "enter":
			return options[selected], nil
		default:
			digit, err := strconv.Atoi(key)
			if err == nil && digit >= 1 && digit <= len(options) {
				return options[digit-1], nil
			}

			continue
		}

		// Up as many lines as the list is long, then rewrite it: nothing scrolls, so the question
		// stays where it was
		fmt.Fprintf(os.Stdout, "\033[%dA", len(options))
		fmt.Fprint(os.Stdout, strings.ReplaceAll(renderOptions(selected, options), "\n", "\r\n"))
	}
}

// readKey names one keypress. An escape sequence arrives as three bytes and anything else as one.
func readKey(reader *bufio.Reader) string {
	first, err := reader.ReadByte()
	if err != nil {
		return "enter"
	}

	switch first {
	case '\r', '\n':
		return "enter"
	case 3:
		// Ctrl-C in raw mode is a byte rather than a signal, and it has to keep meaning what it
		// means everywhere else
		fmt.Fprint(os.Stdout, "\033[?25h")
		os.Exit(130)
	case 27:
		if reader.Buffered() < 2 {
			return "esc"
		}

		bracket, _ := reader.ReadByte()
		final, _ := reader.ReadByte()

		if bracket != '[' && bracket != 'O' {
			return "esc"
		}

		switch final {
		case 'A':
			return "up"
		case 'B':
			return "down"
		case 'C':
			return "right"
		case 'D':
			return "left"
		}

		return "esc"
	}

	return string(first)
}

// numbered is the list that works anywhere: what bash's own `select` draws, and what a terminal
// that cannot be drawn on gets.
func numbered(question string, options []string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Fprint(os.Stdout, section("✅ "+question+"\n"))

		for at, option := range options {
			fmt.Fprintf(os.Stdout, "%d) %s\n", at+1, table(option))
		}

		fmt.Fprint(os.Stdout, "Option: ")

		answer, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(answer) == "" {
			return "", errNothingChosen
		}

		answer = strings.TrimSpace(answer)

		if digit, err := strconv.Atoi(answer); err == nil && digit >= 1 && digit <= len(options) {
			return options[digit-1], nil
		}

		for _, option := range options {
			if answer == option {
				return option, nil
			}
		}

		fmt.Fprint(os.Stdout, warning("\nInvalid option, choose an option\n"))
	}
}

// move is the next index, wrapping at both ends. Somebody at the last option pressing down means
// the first one, not a beep.
func move(index, delta, count int) int {
	if count <= 0 {
		return 0
	}

	index = (index + delta) % count
	if index < 0 {
		index += count
	}

	return index
}

// renderOptions is the list as it appears, one option per line.
//
// The marker is the first thing on the line rather than a colour, because a colour is what a
// monochrome terminal swallows.
func renderOptions(selected int, options []string) string {
	drawn := strings.Builder{}

	for at, option := range options {
		if at == selected {
			drawn.WriteString(fmt.Sprintf("  ❯ %s%d) %s%s\n",
				code("\033[1m"), at+1, option, code(reset)))

			continue
		}

		drawn.WriteString(fmt.Sprintf("    %d) %s\n", at+1, option))
	}

	return drawn.String()
}
