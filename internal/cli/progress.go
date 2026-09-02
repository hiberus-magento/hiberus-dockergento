package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

//
// Saying that something is still happening.
//
// The decision of whether to animate is the shell implementation's, condition for condition: a
// terminal, text output, colour allowed, nobody asking for silence, and a TERM that can draw. Get
// it wrong in either direction and the output stops matching — a spinner in a log file, or a
// command that looks hung for a minute.
//

// begin announces a step and returns the function that ends it.
func begin(label string, stdout, stderr io.Writer, jsonOutput bool) func(ok bool, note string) {
	// In JSON mode stdout carries the document, so anything decorative goes to stderr
	where := stdout
	if jsonOutput {
		where = stderr
	}

	started := time.Now()

	if !animates(jsonOutput) {
		fmt.Fprint(where, good(label+"\n"))

		return func(ok bool, note string) { finish(where, label, note, started, ok) }
	}

	stop := make(chan struct{})

	var waiting sync.WaitGroup

	waiting.Add(1)

	go func() {
		defer waiting.Done()

		frames := spinner()

		for at := 0; ; at = (at + 1) % len(frames) {
			select {
			case <-stop:
				// Erase the line before whatever is printed next lands on top of it
				fmt.Fprint(where, "\r\033[2K")

				return
			case <-time.After(100 * time.Millisecond):
				fmt.Fprintf(where, "\r\033[2K%s %s", frames[at], label)
			}
		}
	}()

	return func(ok bool, note string) {
		close(stop)
		waiting.Wait()
		finish(where, label, note, started, ok)
	}
}

func finish(where io.Writer, label, note string, started time.Time, ok bool) {
	elapsed := ""

	// Under two seconds the number is noise: everything takes a moment
	if seconds := int(time.Since(started).Seconds()); seconds >= 2 {
		elapsed = " (" + duration(seconds) + ")"
	}

	if !ok {
		fmt.Fprint(where, warning(label+" failed"+elapsed+"\n"))

		return
	}

	if note == "" {
		note = "done"
	}

	fmt.Fprint(where, good(label+" "+note+elapsed+"\n"))
}

func duration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	return fmt.Sprintf("%dm%02ds", seconds/60, seconds%60)
}

func animates(jsonOutput bool) bool {
	if jsonOutput || !isTerminal(os.Stdout) {
		return false
	}

	for _, quiet := range []string{"NO_COLOR", "HM_NO_COLOR", "HM_NON_INTERACTIVE", "HM_NO_PROGRESS"} {
		if os.Getenv(quiet) != "" {
			return false
		}
	}

	switch os.Getenv("TERM") {
	case "", "dumb":
		return false
	}

	return true
}

// spinner is braille where the terminal can draw it and dashes where it cannot, which is the same
// thing the shell implementation decides from the same variables.
func spinner() []string {
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LC_CTYPE")
	}

	if locale == "" {
		locale = os.Getenv("LANG")
	}

	if strings.Contains(strings.ToLower(locale), "utf-8") ||
		strings.Contains(strings.ToLower(locale), "utf8") {
		return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}

	return []string{"|", "/", "-", "\\"}
}
