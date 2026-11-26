package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// transferSSHOptions make a connection that is gone fail instead of waiting forever: no answer
// to connect within 30 seconds, or no answer from the server for two minutes once connected.
// Neither touches authentication — BatchMode would have, and a key with a passphrase and no
// agent would then fail where it now asks.
var transferSSHOptions = []string{"-o", "ConnectTimeout=30", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=4"}

// transferAnimates is whether a step that takes a while is drawn while it runs: on a terminal
// that is not under --json, NO_COLOR or HM_NO_PROGRESS, as the spinner of the other commands. A
// variable so that a test can draw without a terminal.
var transferAnimates = func(stderr io.Writer, jsonOutput bool) bool {
	return animates(jsonOutput) && isTerminal(stderr)
}

// transferTick is how often the line is redrawn.
var transferTick = 100 * time.Millisecond

// working draws label on out, with a spinner, until the returned function is called, which
// erases it. An empty label draws nothing: ssh may be asking for a passphrase on the same
// terminal, and a line redrawn over the question would hide it.
func working(out io.Writer, label func(elapsed time.Duration) string) func() {
	stop := make(chan struct{})
	started := time.Now()

	var (
		waiting sync.WaitGroup
		drawn   bool
	)

	waiting.Add(1)

	go func() {
		defer waiting.Done()

		frames := spinner()

		for at := 0; ; at = (at + 1) % len(frames) {
			select {
			case <-stop:
				if drawn {
					fmt.Fprint(out, "\r\033[2K")
				}

				return
			case <-time.After(transferTick):
				if text := label(time.Since(started)); text != "" {
					fmt.Fprintf(out, "\r\033[2K%s %s", frames[at], text)
					drawn = true
				}
			}
		}
	}()

	return func() {
		close(stop)
		waiting.Wait()
	}
}

// stepLabel is a step and how long it has taken, once it has taken long enough to wonder.
func stepLabel(step string, elapsed time.Duration) string {
	if elapsed < time.Second {
		return ""
	}

	return fmt.Sprintf("%s (%s)", step, duration(int(elapsed.Seconds())))
}

// downloadLabel is how much of the dump has arrived over ssh, read from the file it is writing.
// Nothing has arrived until ssh is connected and authenticated.
func downloadLabel(file string) func(time.Duration) string {
	return func(elapsed time.Duration) string {
		info, err := os.Stat(file)
		if err != nil || info.Size() == 0 {
			return ""
		}

		return fmt.Sprintf("Downloading the dump: %s received (%s)", humanBytes(info.Size()),
			duration(int(elapsed.Seconds())))
	}
}
