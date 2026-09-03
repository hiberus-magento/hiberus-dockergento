package cli

import "os"

// The palette, decided the way the shell implementation decides it: what the user asked for beats
// the environment, and nothing is coloured when nobody is looking.
//
// It matters beyond taste. The two implementations are compared byte for byte by a test, and a
// stray escape sequence is a difference — so the rule has to be the same rule, not a similar one.
func coloured() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("HM_NO_COLOR") != "" {
		return false
	}

	switch os.Getenv("TERM") {
	case "", "dumb":
		return false
	}

	if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}

	return isTerminal(os.Stdout)
}

func paint(code, text string) string {
	if !coloured() {
		return text
	}

	return code + text + "\033[0m"
}

// rule is what the shell implementation draws a title between: forty equals signs, no more and no
// fewer, because the two are compared character for character.
const rule = "========================================"

// prompt is a question and the answer it would take if nobody types one.
//
// Painted the way the shell implementation paints it, bracket by bracket: the default is printed
// without the question's colour, which is why this is not three calls to paint.
func prompt(text, suggestion string) string {
	asked := paint(blue, text)

	if suggestion == "" || suggestion == "null" {
		return asked
	}

	return asked + code(blue) + "[" + code(reset) + suggestion + code(blue) + "] " + code(reset)
}

const (
	blue  = "\033[0;34m"
	reset = "\033[0m"
)

// code is an escape sequence when anybody is looking, and nothing when not.
func code(sequence string) string {
	if !coloured() {
		return ""
	}

	return sequence
}

func header(text string) string  { return paint("\033[1;37m", text) }
func good(text string) string    { return paint("\033[0;32m", text) }
func bad(text string) string     { return paint("\033[0;31m", text) }
func section(text string) string { return paint(blue, text) }
func warning(text string) string { return paint("\033[0;33m", text) }

// table is what a menu's options are painted in, which is the one place this colour is used.
func table(text string) string { return paint("\033[0;36m", text) }
func link(text string) string  { return paint("\033[34;4m", text) }
