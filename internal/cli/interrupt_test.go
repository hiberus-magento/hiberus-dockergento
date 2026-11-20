package cli

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

//
// Ctrl-C while a tunnel is being opened or held. The relay is public, or a port of this machine,
// so leaving it behind is the one thing an interruption must not do — and closing it before it
// exists does nothing, which is why the opening phase closes it again once the opening is over.
//

type interrupted struct {
	mu     sync.Mutex
	closed int
	exited chan int
}

func interruptible(t *testing.T) (*interruption, chan os.Signal, *interrupted) {
	t.Helper()

	signals := make(chan os.Signal, 1)
	seen := &interrupted{exited: make(chan int, 1)}

	original, originalExit := notifyInterrupt, exitProcess
	notifyInterrupt = func() <-chan os.Signal { return signals }
	exitProcess = func(code int) { seen.exited <- code }

	t.Cleanup(func() { notifyInterrupt, exitProcess = original, originalExit })

	return closeOnInterrupt(func() {
		seen.mu.Lock()
		seen.closed++
		seen.mu.Unlock()
	}), signals, seen
}

func (s *interrupted) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func TestAnInterruptionBeforeOpeningEndsAtOnce(t *testing.T) {
	_, signals, seen := interruptible(t)

	signals <- syscall.SIGINT

	select {
	case code := <-seen.exited:
		// Nothing was opened by this process: what is there was opened by another terminal
		if code != 130 || seen.count() != 0 {
			t.Errorf("exited %d after %d closes, want 130 after none", code, seen.count())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an interruption at the question did not end the process")
	}
}

func TestAnInterruptionWhileOpeningClosesAgainOnceTheOpeningIsOver(t *testing.T) {
	stop, signals, seen := interruptible(t)

	stop.opening()
	signals <- syscall.SIGINT

	// Closed straight away, then nothing until the opening returns
	time.Sleep(50 * time.Millisecond)

	select {
	case <-seen.exited:
		t.Fatal("ended before the opening was over, when a relay being created could still appear")
	default:
	}

	stop.opened()

	select {
	case code := <-seen.exited:
		if code != 130 || seen.count() != 2 {
			t.Errorf("exited %d after %d closes, want 130 after two", code, seen.count())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not end once the opening was over")
	}
}

func TestAnInterruptionWhileHoldingOnlyCloses(t *testing.T) {
	stop, signals, seen := interruptible(t)

	stop.opening()
	stop.opened()
	stop.holding()
	signals <- syscall.SIGINT

	deadline := time.Now().Add(2 * time.Second)
	for seen.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if seen.count() != 1 {
		t.Fatalf("closed %d times, want once", seen.count())
	}

	// The wait it ends returns, and what follows it says the tunnel closed
	select {
	case code := <-seen.exited:
		t.Errorf("exited %d while holding, want the command to finish on its own", code)
	case <-time.After(100 * time.Millisecond):
	}
}
