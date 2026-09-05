package cli

import (
	"io"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// wrappers_test.go asks these questions of what the copy and varnish handlers reach for. The
// fake below is what answers them: an ordered log of what was asked, and — where a test needs
// one — an outcome to answer it with.
//

// call is one thing asked of a fakeEngine, in the order it was asked. Every commands method
// records one, so a whole test compares against a single log rather than per-method slices,
// which is what makes an interleaving like varnish's own visible in a diff.
type call struct {
	Method   string
	Dir      string
	Service  string
	Command  []string
	Options  core.ExecOptions
	Paths    []string
	All      bool
	Services []string
	Key      string
}

// outcome is what a call answers with. outcomes is indexed by call number — the position the
// call lands at in the log — and a call past the end of outcomes succeeds, so a test states only
// the calls that must fail.
type outcome struct {
	status int
	err    error
}

// fakeEngine is a commands that never reaches Docker: everything asked of it lands in calls, and
// nothing beyond outcomes decides what it answers.
type fakeEngine struct {
	calls    []call
	outcomes []outcome

	// project is what Resolve answers with, when its own outcome does not fail it. Left at its
	// zero value, Name is empty — which is what a directory with no project looks like to the
	// handlers that call it.
	project core.Project

	// properties is what Property answers from, keyed the same way the real engine's own
	// properties are. A key left out of the map answers "", which is what drives the fallbacks
	// wrappers.go falls back to when a project never set one.
	properties map[string]string
}

var _ commands = (*fakeEngine)(nil)

func (f *fakeEngine) outcomeFor(number int) outcome {
	if number < len(f.outcomes) {
		return f.outcomes[number]
	}

	return outcome{}
}

func (f *fakeEngine) Resolve(dir string) (core.Project, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Resolve", Dir: dir})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Project{}, out.err
	}

	return f.project, nil
}

func (f *fakeEngine) Exec(dir, service string, command []string, options core.ExecOptions) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Exec", Dir: dir, Service: service, Command: command, Options: options})

	out := f.outcomeFor(number)

	return out.status, out.err
}

func (f *fakeEngine) Restart(dir string, services []string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Restart", Dir: dir, Services: services})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) CopyInto(dir string, paths []string, all bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CopyInto", Dir: dir, Paths: paths, All: all})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) CopyFrom(dir string, paths []string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CopyFrom", Dir: dir, Paths: paths})

	return f.outcomeFor(number).err
}

// Property answers from f.properties, not from an outcome: it is one of the two things asked of
// the engine that only ever answers, never fails — the same as the real engine, whose reader over
// two files answers "" for a project that never set the key.
func (f *fakeEngine) Property(project core.Project, key string) string {
	f.calls = append(f.calls, call{Method: "Property", Dir: project.Root, Key: key})

	return f.properties[key]
}

// answering substitutes fake for newEngine for the rest of this test, and restores the real
// factory when it ends. HM_STATE_DIR is pinned to a temporary directory first: while a call site
// is still unrouted it reaches the real engine(), and that engine opens its registry at
// HM_STATE_DIR — unset, that is the developer's own ~/.hm.
func answering(t *testing.T) *fakeEngine {
	t.Helper()

	t.Setenv("HM_STATE_DIR", t.TempDir())

	fake := &fakeEngine{}

	original := newEngine
	newEngine = func(_, _ io.Writer, _ bool) commands { return fake }
	t.Cleanup(func() { newEngine = original })

	return fake
}
