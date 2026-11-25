package cli

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// How far a copy between this machine and a container has got.
//
// A vendor directory is tens of thousands of files, and a copy of it that says nothing for a
// minute looks hung. The line goes to stderr and only to a terminal: a log, a pipe, CI and --json
// are given no watch at all, so what they read is byte for byte what it was before.
//

// copyTerminal is the seam a test substitutes to stand in for a terminal.
var copyTerminal = isTerminal

// copyWatch is what the engine is handed to show copies, or nil when nobody is watching.
func copyWatch(stderr io.Writer, jsonOutput bool) ports.CopyWatch {
	if jsonOutput || !copyTerminal(stderr) {
		return nil
	}

	for _, quiet := range []string{"CI", "HM_NO_PROGRESS", "HM_NON_INTERACTIVE"} {
		if os.Getenv(quiet) != "" {
			return nil
		}
	}

	// A carriage return is all the line needs, but a terminal that cannot draw is usually not a
	// terminal anybody is looking at, which is the same reading the spinner makes
	switch os.Getenv("TERM") {
	case "", "dumb":
		return nil
	}

	// NO_COLOR asks for no colour, not for no progress: the bar is characters, not escapes
	colour := os.Getenv("NO_COLOR") == "" && os.Getenv("HM_NO_COLOR") == ""

	return func(total int64) (ports.CopyMeter, func()) {
		return drawCopy(stderr, total, colour)
	}
}

// copyRedraw is how often the line is drawn at most: often enough to look alive, seldom enough
// that a copy of tens of thousands of small files does not spend its time painting.
const copyRedraw = 100 * time.Millisecond

// drawCopy starts the line for one copy, and returns what counts it and what ends it.
//
// The counts are kept in atomics and drawn by a ticker, never by the copy itself, so the meter
// costs the copy two stores however often it is called.
func drawCopy(out io.Writer, total int64, colour bool) (ports.CopyMeter, func()) {
	line := &copyLine{out: out}
	started := time.Now()

	var carried, files atomic.Int64

	state := func(done bool) copyState {
		return copyState{
			bytes: carried.Load(), files: files.Load(), total: total,
			elapsed: time.Since(started), done: done,
		}
	}

	stop := make(chan struct{})

	var drawing sync.WaitGroup

	drawing.Add(1)

	go func() {
		defer drawing.Done()

		ticker := time.NewTicker(copyRedraw)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				line.draw(formatCopy(state(false), colour))
			}
		}
	}()

	meter := func(bytes, count int64) {
		carried.Store(bytes)
		files.Store(count)
	}

	var once sync.Once

	end := func() {
		once.Do(func() {
			close(stop)
			drawing.Wait()
			line.draw(formatCopy(state(true), colour))
			fmt.Fprint(out, "\n")
		})
	}

	return meter, end
}

// copyLine is one line drawn over itself.
type copyLine struct {
	out   io.Writer
	width int
}

// draw replaces the line, padding over whatever a longer one left: spaces rather than an erase
// sequence, so that without colour nothing but text and a carriage return is written.
func (l *copyLine) draw(text string) {
	width := copyWidth(text)
	pad := ""

	if l.width > width {
		pad = strings.Repeat(" ", l.width-width)
	}

	l.width = width

	fmt.Fprint(l.out, "\r"+text+pad)
}

// copyWidth is how many columns a line takes, not counting its colour sequences.
func copyWidth(text string) int {
	width := 0

	for at := 0; at < len(text); {
		if text[at] == '\033' {
			if end := strings.IndexByte(text[at:], 'm'); end >= 0 {
				at += end + 1

				continue
			}
		}

		_, size := utf8.DecodeRuneInString(text[at:])
		at += size
		width++
	}

	return width
}

// copyState is how far a copy has got, as the line shows it.
type copyState struct {
	bytes, files int64

	// total is the bytes expected, 0 when nobody knows
	total int64

	elapsed time.Duration
	done    bool
}

const copyBar = 20

// formatCopy is the line for a copy: a bar, a percentage and a time left when the total is known;
// a counter when it is not.
func formatCopy(state copyState, colour bool) string {
	parts := []string{}
	total := state.total

	// A finished copy carried exactly what it carried: the measured total was an estimate (du
	// counts directories, a file may have changed meanwhile), and a copy that is done is 100%
	if total > 0 && (state.done || state.bytes > total) {
		total = state.bytes
	}

	head := "  "

	if total > 0 {
		percent := int64(100)
		if state.bytes < total {
			percent = state.bytes * 100 / total
		}

		filled := int(percent) * copyBar / 100
		bar := strings.Repeat("█", filled)

		// Painted here and not with good(), which asks whether stdout is a terminal: this line
		// is on stderr, and whether it is drawn at all was already decided
		if colour && filled > 0 {
			bar = "\033[0;32m" + bar + "\033[0m"
		}

		head += bar + strings.Repeat("░", copyBar-filled) + fmt.Sprintf(" %3d%%  ", percent)
		parts = append(parts, humanBytes(state.bytes)+" / "+humanBytes(total))
	} else {
		parts = append(parts, humanBytes(state.bytes))
	}

	if state.files == 1 {
		parts = append(parts, "1 file")
	} else {
		parts = append(parts, fmt.Sprintf("%d files", state.files))
	}

	seconds := state.elapsed.Seconds()
	rate := 0.0

	if seconds > 0 {
		rate = float64(state.bytes) / seconds
		parts = append(parts, humanBytes(int64(rate))+"/s")
	}

	switch {
	case state.done:
		parts = append(parts, duration(int(seconds)))
	case total > 0 && rate > 0 && state.bytes < total:
		parts = append(parts, "~"+duration(int(math.Ceil(float64(total-state.bytes)/rate))))
	}

	return head + strings.Join(parts, " · ")
}

// humanBytes is a size the way a person reads it, in powers of a thousand: 412 MB, 1.5 GB.
func humanBytes(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}

	units := []string{"kB", "MB", "GB", "TB", "PB"}
	value := float64(n)

	for at, unit := range units {
		value /= 1000

		// The thresholds are where the rounding would print 10.0 or 1000, which belong to the
		// next format and the next unit
		if value < 9.95 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}

		if value < 999.5 || at == len(units)-1 {
			return fmt.Sprintf("%.0f %s", value, unit)
		}
	}

	return ""
}
