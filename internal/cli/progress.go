package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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

// reporting is what the engine is handed to say what its long steps are doing: Announce for a
// step, Progress for one that is timed and ends.
func reporting(stdout, stderr io.Writer, jsonOutput bool) (func(string), func(string) func(bool, string)) {
	if os.Getenv("HM_PROGRESS") == "jsonl" {
		return progressEvents(stderr)
	}

	// In JSON mode stdout carries the document, so anything decorative goes to stderr: otherwise
	// a program reading the output finds a sentence in the middle of it
	announce := func(message string) {
		where := stdout
		if jsonOutput {
			where = stderr
		}

		fmt.Fprint(where, good(message))
	}

	// The spinner lives here because the engine has no terminal and should not learn about one
	progress := func(label string) func(bool, string) {
		return begin(label, stdout, stderr, jsonOutput)
	}

	return announce, progress
}

//
// --progress=jsonl: the same steps, for a program. One JSON object per line on stderr, and nothing
// else of ours there while it lasts: a line of a spinner or a sentence in between would be a line
// the reader cannot parse.
//

// progressStarted and progressCommand are the invocation the events belong to, set by globals:
// elapsed counts from the start of the command, not from the engine a handler happened to build
// for one of its steps.
var (
	progressStarted = time.Now()
	progressCommand = ""
)

// progressEvent is one line. Pct is a pointer because unknown is null, not 0: none of the steps
// the tool takes today knows how far through it is.
type progressEvent struct {
	Phase   string   `json:"phase"`
	Step    string   `json:"step"`
	Pct     *float64 `json:"pct"`
	Elapsed float64  `json:"elapsed"`
}

func progressEvents(stderr io.Writer) (func(string), func(string) func(bool, string)) {
	var writing sync.Mutex

	emit := func(step string) {
		line, err := json.Marshal(progressEvent{
			Phase: progressCommand, Step: step,
			Elapsed: math.Round(time.Since(progressStarted).Seconds()*10) / 10,
		})
		if err != nil {
			return
		}

		writing.Lock()
		defer writing.Unlock()

		fmt.Fprintf(stderr, "%s\n", line)
	}

	// An announcement is a sentence with its own line breaks, sometimes two: one event per line
	// of it that says something
	announce := func(message string) {
		for _, line := range strings.Split(message, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				emit(line)
			}
		}
	}

	// A timed step is two events, the same words the text says: the label, then how it ended
	progress := func(label string) func(bool, string) {
		emit(label)

		return func(ok bool, note string) {
			switch {
			case !ok:
				note = "failed"
			case note == "":
				note = "done"
			}

			emit(label + " " + note)
		}
	}

	return announce, progress
}

// progressPhase is what the events call the command: its name, and for the commands that are a
// family of subcommands, the subcommand too — "db snapshot" and "db restore" are not one phase.
func progressPhase(rest []string) string {
	words := []string{}

	for _, argument := range rest {
		if strings.HasPrefix(argument, "-") {
			continue
		}

		words = append(words, argument)

		if len(words) == 2 || (words[0] != "db" && words[0] != "worktree") {
			break
		}
	}

	return strings.Join(words, " ")
}

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
