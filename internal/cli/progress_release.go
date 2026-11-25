package cli

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// What update and switch are doing while they do it: looking the release up, downloading it,
// checking it and putting it in place, each a spinner while it runs and a tick once it is done,
// the way the installer shows its steps.
//
// It goes to stderr and only where the copy line goes: a log, a pipe, CI and --json get no watch,
// so what they read, and the summary on stdout everywhere, is byte for byte what it was.
//

// releaseWatch is what the engine is handed to show a move to another release, or nothing when
// nobody is watching.
func releaseWatch(stderr io.Writer, jsonOutput bool) ports.ReleaseWatch {
	colour, drawn := drawsOn(stderr, jsonOutput)
	if !drawn {
		return ports.ReleaseWatch{}
	}

	return ports.ReleaseWatch{
		Step: func(label string) func(string, bool) {
			return drawStep(stderr, label, colour)
		},
		Download: func(total int64) (ports.CopyMeter, func(bool)) {
			return drawDownload(stderr, total, colour)
		},
	}
}

// drawStep draws label behind a spinner until the step ends, and leaves what it ended as: a tick
// and what it found, a cross and its own words when it failed, or nothing at all.
func drawStep(out io.Writer, label string, colour bool) func(done string, failed bool) {
	line := &copyLine{out: out}
	frames := spinner()

	// The first frame at once: a step drawn only after a tick is a tenth of a second of the
	// silence this is here to end, and a step quicker than that would never be seen running
	line.draw(frames[0] + " " + label)

	stop := make(chan struct{})

	var drawing sync.WaitGroup

	drawing.Add(1)

	go func() {
		defer drawing.Done()

		ticker := time.NewTicker(copyRedraw)
		defer ticker.Stop()

		for at := 1; ; at = (at + 1) % len(frames) {
			select {
			case <-stop:
				return
			case <-ticker.C:
				line.draw(frames[at] + " " + label)
			}
		}
	}()

	var once sync.Once

	return func(done string, failed bool) {
		once.Do(func() {
			close(stop)
			drawing.Wait()

			switch {
			case failed:
				line.draw(marked("✗", "\033[0;31m", colour) + " " + label)
				fmt.Fprint(out, "\n")
			case done == "":
				// Padded over and the cursor put back: whatever is printed next takes the line
				line.draw("")
				fmt.Fprint(out, "\r")
			default:
				line.draw(marked("✓", "\033[0;32m", colour) + " " + done)
				fmt.Fprint(out, "\n")
			}
		})
	}
}

// marked is a step's mark, in its colour when there is colour. Painted here and not with good(),
// which asks whether stdout is a terminal: this line is on stderr, and that was already decided.
func marked(mark, paint string, colour bool) string {
	if !colour {
		return mark
	}

	return paint + mark + "\033[0m"
}
