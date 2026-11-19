package cli

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

//
// Ctrl-C, or a terminal going away, while share or tunnel has something open.
//
// What is open is a container on the daemon: a public tunnel to the environment, or a port of this
// machine. A process that dies without closing it leaves it there, so the interruption is taken
// from before anything is opened, in three phases:
//
//   - before opening, at the question: nothing to close yet, and the process ends at once
//   - while opening: the relay may be in the middle of being created, so it is closed now and
//     again once the opening returns, and then the process ends
//   - while holding: it is closed, which ends the wait, and the command finishes on its own
//

// openingGrace bounds how long an interruption waits for an opening to return before closing
// again and ending anyway.
const openingGrace = 15 * time.Second

// notifyInterrupt and exitProcess are what a test substitutes.
var (
	notifyInterrupt = func() <-chan os.Signal {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

		return signals
	}
	exitProcess = os.Exit
)

const (
	phaseAsking int32 = iota
	phaseOpening
	phaseHolding
)

type interruption struct {
	phase atomic.Int32
	done  chan struct{}
}

// closeOnInterrupt runs close on the first interruption, in whatever way the phase asks.
func closeOnInterrupt(close func()) *interruption {
	state := &interruption{done: make(chan struct{})}

	signals := notifyInterrupt()

	go func() {
		<-signals

		close()

		switch state.phase.Load() {
		case phaseHolding:
			return
		case phaseOpening:
			select {
			case <-state.done:
			case <-time.After(openingGrace):
			}

			close()
		}

		exitProcess(130)
	}()

	return state
}

func (i *interruption) opening() { i.phase.Store(phaseOpening) }

func (i *interruption) opened() {
	select {
	case <-i.done:
	default:
		close(i.done)
	}
}

func (i *interruption) holding() { i.phase.Store(phaseHolding) }
