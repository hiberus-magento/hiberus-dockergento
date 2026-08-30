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

func header(text string) string  { return paint("\033[1;37m", text) }
func section(text string) string { return paint("\033[0;34m", text) }
func warning(text string) string { return paint("\033[0;33m", text) }
func link(text string) string    { return paint("\033[34;4m", text) }
