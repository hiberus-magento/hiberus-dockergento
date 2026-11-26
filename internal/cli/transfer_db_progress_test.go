package cli

import (
	"bytes"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

//
// What transfer-db shows while it waits. The user's run looked hung: a dump over SSH, a copy,
// a restore, none of which said anything until it ended. Whatever can take minutes now says it is
// still going, and nothing waits forever on a connection that is gone.
//

// sshOptions are the options every ssh runs with: they make a dead connection fail instead of
// waiting forever, and none of them stops ssh from asking for a passphrase or a host key.
func sshOptions() []string {
	return []string{"-o", "ConnectTimeout=30", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=4"}
}

func TestSSHFailsOnADeadConnectionAndStillAsksForAPassphrase(t *testing.T) {
	_, ran := dbLab(t)

	transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false)

	if len(*ran) != 1 {
		t.Fatalf("ssh ran %v", *ran)
	}

	args := (*ran)[0].Args
	if !slices.Equal(args[:len(args)-2], sshOptions()) {
		t.Fatalf("ssh options = %q, want %q", args[:len(args)-2], sshOptions())
	}

	for _, arg := range args {
		if strings.Contains(arg, "BatchMode") || strings.Contains(arg, "StrictHostKeyChecking") {
			t.Fatalf("ssh ran with %q, which stops it asking for a passphrase or a host key", arg)
		}
	}
}

// drawing makes the steps draw as on a terminal, quickly.
func drawing(t *testing.T) {
	t.Helper()

	previousAnimates, previousTick := transferAnimates, transferTick
	t.Cleanup(func() { transferAnimates, transferTick = previousAnimates, previousTick })

	transferAnimates = func(io.Writer, bool) bool { return true }
	transferTick = 2 * time.Millisecond
}

// TestTheDownloadSaysHowMuchHasArrived: the bytes ssh has written so far, drawn on stderr — and
// nothing drawn before the first byte, while ssh may be asking for a passphrase on the terminal.
func TestTheDownloadSaysHowMuchHasArrived(t *testing.T) {
	_, ran := dbLab(t)
	drawing(t)

	runSSH = func(args []string, output string, _ io.Writer) (int, error) {
		*ran = append(*ran, sshRun{Args: args, Output: output})

		// The passphrase: nothing has arrived yet
		time.Sleep(40 * time.Millisecond)

		if err := os.WriteFile(output, bytes.Repeat([]byte("x"), 2500), 0o600); err != nil {
			t.Error(err)
		}

		time.Sleep(40 * time.Millisecond)

		return 0, nil
	}

	var stderr bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--anonymise=false"}, io.Discard, &stderr, false)

	drawn := stderr.String()
	mustContain(t, "stderr", drawn, "Downloading the dump: 2.5 kB received (0s)")
	mustNotContain(t, "stderr", drawn, "0 B received")

	if !strings.HasPrefix(drawn, "\r\033[2K") {
		t.Errorf("stderr = %q, want the line drawn from its start", drawn)
	}
}

// TestALongStepSaysItIsStillRunning: the restore of a large dump takes minutes and prints
// nothing; its name and the time it has taken are redrawn until it ends, then erased.
func TestALongStepSaysItIsStillRunning(t *testing.T) {
	drawing(t)

	var out bytes.Buffer

	stop := working(&out, func(elapsed time.Duration) string {
		return stepLabel("Restoring the dump into the local database", elapsed+90*time.Second)
	})

	time.Sleep(20 * time.Millisecond)
	stop()

	drawn := out.String()
	mustContain(t, "the line", drawn, "Restoring the dump into the local database (1m30s)")

	if !strings.HasSuffix(drawn, "\r\033[2K") {
		t.Errorf("drawn = %q, want the line erased at the end", drawn)
	}
}

// TestAShortStepDrawsNothing: under a second there is nothing to wait for.
func TestAShortStepDrawsNothing(t *testing.T) {
	if label := stepLabel("Saving core_config_data", 300*time.Millisecond); label != "" {
		t.Fatalf("label after 300ms = %q, want nothing", label)
	}
}

// TestNothingIsDrawnWhereNobodyWatches: off a terminal, under --json or in a test, the output
// stays what it was.
func TestNothingIsDrawnWhereNobodyWatches(t *testing.T) {
	_, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{})

	var stderr bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--anonymise=false"}, io.Discard, &stderr, false)

	mustNotContain(t, "stderr", stderr.String(), "\r", "received")
}
