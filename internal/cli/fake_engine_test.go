package cli

import (
	"io"
	"path/filepath"
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
	Path     string
	Domain   string
	Database bool
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

	// installed and tooling are what Installed answers with — the other thing asked of the
	// engine that only ever answers, never fails, the same as Property.
	installed core.Installation
	tooling   core.Tooling
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

// Dump, unlike Property, is asked to do something and can be refused: it consumes an outcome the
// way Resolve and Exec do, rather than only ever answering.
func (f *fakeEngine) Dump(dir, path string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Dump", Dir: dir, Path: path})

	return f.outcomeFor(number).err
}

// Installed answers from f.installed and f.tooling, the same as Property: asked about no
// project, and never refused.
func (f *fakeEngine) Installed() (core.Installation, core.Tooling) {
	f.calls = append(f.calls, call{Method: "Installed"})

	return f.installed, f.tooling
}

// SetHost, like Dump, consumes an outcome and can be refused.
func (f *fakeEngine) SetHost(dir, domain string, database bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "SetHost", Dir: dir, Domain: domain, Database: database})

	return f.outcomeFor(number).err
}

// RemoveHost, unlike SetHost, asks nothing about a project: there is no dir to record.
func (f *fakeEngine) RemoveHost(domain string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "RemoveHost", Domain: domain})

	return f.outcomeFor(number).err
}

// answering substitutes fake for newEngine for the rest of this test, and restores the real
// factory when it ends. Six pins go first, unconditionally, because while a call site is still
// unrouted — the RED half of a test written against this helper — it reaches the real engine()
// and everything underneath it:
//
//   - HM_STATE_DIR: the real engine() opens its registry there; unset, that is the developer's
//     own ~/.hm.
//   - HM_HOSTS_FILE: Hosts.file() falls back to the literal /etc/hosts (app/hosts.go:186), and
//     write() copies over it through `sudo cp` wired to the real terminal (app/hosts.go:173).
//     Pointed at a temporary file that does not exist, Set and Remove both fail at os.ReadFile
//     before anything is written.
//   - HM_NON_INTERACTIVE: choose() refuses immediately once this is set (select.go:39),
//     independently of any interactive bool, so a RED run never blocks on a question.
//   - DOCKER_HOST: an unrouted call into toolinfo's DockerVersion dials with no deadline of its
//     own; pointed at a socket that cannot exist, the dial fails fast instead of hanging.
//   - HM_LEGACY_ROOT: reached only when Hosts.Set's own resolution falls through to the legacy
//     branch, which execs <root>/bin/run (legacy/runner.go:94) when ShellRoot is empty. A
//     developer's own HM_LEGACY_ROOT would have a RED run shell out to the Bash half; a fresh
//     temporary directory makes that exec fail with ENOENT instead.
//   - t.Chdir(t.TempDir()): app/hosts.go:49 writes properties.json under the resolved project
//     root, which without this pin is the directory the test binary happens to run in — the
//     package's own checkout.
func answering(t *testing.T) *fakeEngine {
	t.Helper()

	t.Setenv("HM_STATE_DIR", t.TempDir())
	t.Setenv("HM_HOSTS_FILE", filepath.Join(t.TempDir(), "hosts"))
	t.Setenv("HM_NON_INTERACTIVE", "1")
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HM_LEGACY_ROOT", t.TempDir())
	t.Chdir(t.TempDir())

	fake := &fakeEngine{}

	original := newEngine
	newEngine = func(_, _ io.Writer, _ bool) commands { return fake }
	t.Cleanup(func() { newEngine = original })

	return fake
}
