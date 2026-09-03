// Package e2e runs the tool the way somebody runs it: the built binary, in a project on disk,
// against a real Docker.
//
// It exists because the tool is a program, and what a program does is not the sum of what its
// functions return. The unit tests underneath say that a compose file is rendered correctly; these
// say that `hm setup` writes it where the next command looks.
//
// What is *not* here is the shell implementation. While both halves existed, the tests that
// mattered most compared them byte for byte, and those live in tests/integration as shell scripts
// because half their inputs are shell functions. As each command loses its shell half, its test
// comes here and the script goes.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

//
// The binary, built once for the whole package.
//
// Once, and not once per test: building it is two seconds and there is no version of this where a
// test needs a different build from its neighbour.
//

var (
	binary     string
	buildOnce  sync.Once
	buildError error
)

// Binary is the built tool, or a skip when this machine cannot build it.
func Binary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		root, err := repositoryRoot()
		if err != nil {
			buildError = err

			return
		}

		target := filepath.Join(os.TempDir(), "hm-e2e", "hm")

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			buildError = err

			return
		}

		build := exec.Command("go", "build", "-o", target, "./cmd/hm")
		build.Dir = root

		if output, err := build.CombinedOutput(); err != nil {
			buildError = fmt.Errorf("no se pudo construir el binario: %v\n%s", err, output)

			return
		}

		binary = target
	})

	if buildError != nil {
		t.Skipf("saltada: %v", buildError)
	}

	return binary
}

// repositoryRoot walks up from this file's package to the module root.
func repositoryRoot() (string, error) {
	here, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(here, "go.mod")); err == nil {
			return here, nil
		}

		here = filepath.Dir(here)
	}

	return "", fmt.Errorf("no se encontró la raíz del módulo")
}

//
// Running it.
//

// Result is what an invocation did.
type Result struct {
	Code   int
	Stdout string
	Stderr string
}

// Output is everything it said, which is what an assertion about a message usually wants.
func (r Result) Output() string { return r.Stdout + r.Stderr }

// Session is one machine, as far as the tool is concerned: a home of its own, a registry of its
// own, its own database copies and its own hosts file.
//
// It is per test rather than per invocation because most of what this tool does is remembered — a
// branch environment registered by one command is listed by the next — and because a test that can
// reach the real machine will eventually write to it. What is at stake there is the registry every
// project on it shares, the only copies of somebody's databases, and the proxy everything routes
// through.
type Session struct {
	// Home is where everything this tool keeps on a machine is kept instead.
	Home string

	// HostsFile is the machine's name resolution, redirected: what `set-host` writes to.
	HostsFile string

	extra []string
}

// New is a session of its own for this test.
func New(t *testing.T) *Session {
	t.Helper()

	home := t.TempDir()

	return &Session{Home: home, HostsFile: filepath.Join(home, "hosts")}
}

// With adds environment entries to every invocation of this session, for the test that needs one.
func (s *Session) With(entries ...string) *Session {
	s.extra = append(s.extra, entries...)

	return s
}

// Run runs the tool in a directory and returns what happened, rather than failing the test: what a
// command answers when it refuses is as much the subject of a test as what it answers when it
// works.
func (s *Session) Run(t *testing.T, dir string, args ...string) Result {
	t.Helper()

	command := exec.Command(Binary(t), args...)
	command.Dir = dir
	command.Env = append(os.Environ(), s.environment()...)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command.Stdout, command.Stderr = stdout, stderr

	err := command.Run()

	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}

	var exit *exec.ExitError
	if err != nil {
		if !asExit(err, &exit) {
			t.Fatalf("no se pudo ejecutar %v: %v", args, err)
		}

		result.Code = exit.ExitCode()
	}

	return result
}

// Reads a file of this session's machine, or the empty string when there is none.
func (s *Session) Reads(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(contents)
}

// Writes a file of this session's machine, which is how the scene is set.
func (s *Session) Writes(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (s *Session) environment() []string {
	return append([]string{
		"HOME=" + s.Home,
		"HM_STATE_DIR=" + filepath.Join(s.Home, ".hm", "state"),
		"HM_WORKTREE_DIR=" + filepath.Join(s.Home, ".hm", "worktrees"),
		"HM_SNAPSHOT_DIR=" + filepath.Join(s.Home, ".hm", "snapshots"),
		"HM_CACHE_DIR=" + filepath.Join(s.Home, ".hm", "cache"),
		"HM_PROXY_DIR=" + filepath.Join(s.Home, ".hm", "proxy"),
		"HM_HOSTS_FILE=" + s.HostsFile,

		// Docker keeps its configuration and its contexts under the real home, so it has to be
		// pointed at them or every test that talks to a daemon loses it
		"DOCKER_CONFIG=" + dockerConfig(),

		// Nothing is coloured: what these compare is text, and an escape sequence is a difference
		// that is invisible in the failure message
		"NO_COLOR=1",
		"TERM=dumb",
	}, s.extra...)
}

func dockerConfig() string {
	if set := os.Getenv("DOCKER_CONFIG"); set != "" {
		return set
	}

	return filepath.Join(realHome(), ".docker")
}

func realHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return home
}

// asExit is errors.As for an exit status, kept here so the callers read as what they are.
func asExit(err error, target **exec.ExitError) bool {
	exit, ok := err.(*exec.ExitError) //nolint:errorlint
	if ok {
		*target = exit
	}

	return ok
}

//
// Reading what came back.
//

// JSON is the document a command answered with, or a failure saying what it answered instead.
func JSON(t *testing.T, output string) map[string]any {
	t.Helper()

	document := map[string]any{}

	if err := json.Unmarshal([]byte(output), &document); err != nil {
		t.Fatalf("no es un documento: %v\n%s", err, output)
	}

	return document
}

// Field is one value out of a document, addressed as `data.project.name`.
func Field(t *testing.T, output, path string) any {
	t.Helper()

	current := any(JSON(t, output))

	for _, step := range strings.Split(path, ".") {
		holder, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s: %s no es un objeto", path, step)
		}

		current, ok = holder[step]
		if !ok {
			return nil
		}
	}

	return current
}

// String is Field as text, which is what most assertions compare.
func String(t *testing.T, output, path string) string {
	t.Helper()

	value := Field(t, output, path)
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}

//
// Standing in for the half that is not ported yet.
//
// Some commands still hand steps to the shell implementation, and what a test about those steps
// has to check is that the right thing was asked for in the right order — not that a Magento
// answered. So the shell tree is replaced by one that writes down what it was told and returns
// success.
//

// Recorder points this session's bridge at a shell tree that records instead of running, and
// answers with what has been asked of it so far.
func Recorder(t *testing.T, session *Session) func() string {
	t.Helper()

	tree := filepath.Join(t.TempDir(), "shell")
	log := filepath.Join(tree, "asked.txt")

	if err := os.MkdirAll(filepath.Join(tree, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	// `console/` is what makes a directory a shell tree, as far as the bridge is concerned
	if err := os.MkdirAll(filepath.Join(tree, "console"), 0o755); err != nil {
		t.Fatal(err)
	}

	script := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> " + log + "\n"

	if err := os.WriteFile(filepath.Join(tree, "bin", "run"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	session.With("HM_LEGACY_ROOT=" + tree)

	return func() string { return session.Reads(log) }
}
