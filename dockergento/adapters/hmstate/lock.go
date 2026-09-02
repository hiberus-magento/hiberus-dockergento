package hmstate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//
// The lock the shell implementation takes, taken the same way.
//
// Not a nicety of interoperability: while both implementations create branch environments, two
// agents — one of each — checking whether a name is free and then claiming it are four moments in
// which the other can do the same thing, and both end up believing they own it. They only
// serialise if they agree on what the lock is, which is a directory of this name with a pid in it.
//
// A directory and not `flock`: macOS does not ship flock(1), and `mkdir` is atomic everywhere.
//

const (
	lockTimeout = 10 * time.Second
	lockPoll    = 100 * time.Millisecond
)

// Lock is one of them.
type Lock struct {
	path string
}

// Take acquires the named lock, waiting for whoever holds it and breaking it when nobody does.
func Take(name string) (*Lock, error) {
	directory := os.Getenv("HM_LOCK_DIR")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		directory = filepath.Join(home, ".hm", "locks")
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(directory, name+".lock")
	deadline := time.Now().Add(lockTimeout)

	for {
		if err := os.Mkdir(path, 0o755); err == nil {
			// With the newline the shell implementation writes, so either can read the other's
			os.WriteFile(filepath.Join(path, "pid"), //nolint:errcheck
				[]byte(strconv.Itoa(os.Getpid())+"\n"), 0o644) //nolint:gosec

			return &Lock{path: path}, nil
		}

		// A lock with no pid, or with a pid nobody answers for, is nobody's: a command killed
		// halfway would otherwise block every other one until somebody found the directory
		if stale(path) {
			os.RemoveAll(path) //nolint:errcheck

			continue
		}

		if time.Now().After(deadline) {
			return nil, errLocked{}
		}

		time.Sleep(lockPoll)
	}
}

// Release gives it back.
func (l *Lock) Release() {
	if l != nil {
		os.RemoveAll(l.path) //nolint:errcheck
	}
}

func stale(path string) bool {
	contents, err := os.ReadFile(filepath.Join(path, "pid"))
	if err != nil {
		return true
	}

	// Trimmed: the shell implementation writes the pid with a newline, and reading it as-is would
	// make every lock it holds look like nobody's — which is the opposite of what this is for
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || pid <= 0 {
		return true
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return true
	}

	return process.Signal(syscall.Signal(0)) != nil
}

type errLocked struct{}

func (errLocked) Error() string {
	return "another worktree is being created; this one waited " + lockTimeout.String()
}
