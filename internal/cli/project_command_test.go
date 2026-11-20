package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// A project's own commands, run the way bin/run ran them. The script is run for real; Docker is
// not: the daemon check and the compose validation are answered by a substitute for spawnProcess,
// and nothing here reaches a daemon.
//

type spawn struct {
	program string
	args    []string
}

type spawner struct {
	docker, compose int
	composeError    string
	calls           []spawn
}

// spawning substitutes the docker programs and lets everything else — the script — really run.
func spawning(t *testing.T, docker, compose int, composeError string) *spawner {
	t.Helper()

	s := &spawner{docker: docker, compose: compose, composeError: composeError}
	original, originalLook := spawnProcess, lookPath

	spawnProcess = func(program string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		s.calls = append(s.calls, spawn{program, args})

		switch {
		case filepath.Base(program) == "docker" && len(args) > 0 && args[0] == "info":
			return s.docker, nil
		case filepath.Base(program) == "docker" && len(args) > 1 && args[0] == "compose" && args[1] == "version":
			return 0, nil
		case len(args) > 1 && args[len(args)-2] == "config" && args[len(args)-1] == "-q":
			io.WriteString(stderr, s.composeError) //nolint:errcheck

			return s.compose, nil
		}

		return original(program, args, env, stdin, stdout, stderr)
	}

	lookPath = func(name string) (string, error) {
		if name == "docker" {
			return "/nonexistent/docker", nil
		}

		return "", exec.ErrNotFound
	}

	t.Setenv("HM_CACHE_DIR", t.TempDir())
	t.Cleanup(func() { spawnProcess, lookPath = original, originalLook })

	return s
}

func (s *spawner) ran(program string) bool {
	for _, call := range s.calls {
		if strings.Contains(call.program, program) {
			return true
		}
	}

	return false
}

// projectWithCommand is a configured project with one command that prints its arguments and the
// variables the test names, and exits with the code it was given in EXIT_WITH.
func projectWithCommand(t *testing.T, dir string, properties string) string {
	t.Helper()

	writeScript(t, filepath.Join(dir, "docker-compose.yml"), "services: {}\n", 0o644)
	writeScript(t, filepath.Join(dir, "docker-compose.dev.mac.yml"), "services: {}\n", 0o644)
	writeScript(t, filepath.Join(dir, "config", "docker", "properties.json"), properties, 0o644)
	writeScript(t, filepath.Join(dir, "config", "hm", "commands", "hello.sh"), `#!/bin/sh
printf 'args=%s\n' "$*"
for name in $SHOW; do eval "printf '%s=%s\n' \"$name\" \"\$$name\""; done
printf 'to stderr\n' >&2
exit "${EXIT_WITH:-0}"
`, 0o755)

	return dir
}

func writeScript(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

// runProject is the entry point from dir.
func runProject(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()

	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)

	return stdout.String(), stderr.String(), code
}

func projectFixture(t *testing.T) (*fakeEngine, *spawner, string) {
	t.Helper()

	fake := answering(t)
	pinHelp(t, helpTool, "mac")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("HM_NON_INTERACTIVE", "")
	t.Setenv("HM_FORCE", "")

	dir, _ := filepath.EvalSymlinks(t.TempDir())
	project := projectWithCommand(t, filepath.Join(dir, "tienda"), `{"COMPOSE_PROJECT_NAME": "tienda", "DOMAIN": "tienda.local"}`)

	return fake, spawning(t, 0, 0, ""), project
}

func TestAProjectCommandRunsWithItsArgumentsAndItsExitCode(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("EXIT_WITH", "7")

	stdout, stderr, code := runProject(t, project, "hello", "uno", "dos tres")

	if code != 7 || stdout != "args=uno dos tres\n" || stderr != "to stderr\n" {
		t.Fatalf("= %d %q %q, want the script's own output and exit code", code, stdout, stderr)
	}
}

func TestTheToolsOwnFlagsAreNotTheScripts(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("SHOW", "HM_OUTPUT_FORMAT HM_NON_INTERACTIVE USE_DEFAULT_SETTINGS")

	stdout, _, _ := runProject(t, project, "hello", "uno", "--json", "--yes", "dos")

	want := "args=uno dos\nHM_OUTPUT_FORMAT=json\nHM_NON_INTERACTIVE=1\nUSE_DEFAULT_SETTINGS=true\n"
	if stdout != want {
		t.Fatalf("got %q\nwant %q", stdout, want)
	}
}

func TestTheScriptGetsTheEnvironmentBinRunExported(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("SHOW", "COMPOSE_PROJECT_NAME HM_PROJECT HM_ROOT HM_IS_WORKTREE HM_COMMAND DOMAIN USER_PHP "+
		"COMMAND_BIN_DIR COMMANDS_DIR CUSTOM_PROPERTIES_DIR MACHINE DOCKER_COMPOSE_FILE_MACHINE DOCKER_COMPOSE "+
		"HM_WORKTREE_ENVIRONMENT HM_PROFILE HM_EXIT_PROJECT GREEN COLOR_RESET HM_OUTPUT_FORMAT")

	stdout, _, _ := runProject(t, project, "hello", "--no-json")

	want := "args=\n" +
		"COMPOSE_PROJECT_NAME=tienda\n" +
		"HM_PROJECT=tienda\n" +
		"HM_ROOT=" + project + "\n" +
		"HM_IS_WORKTREE=false\n" +
		"HM_COMMAND=hello\n" +
		"DOMAIN=tienda.local\n" +
		"USER_PHP=app\n" +
		"COMMAND_BIN_DIR=" + helpTool + "\n" +
		"COMMANDS_DIR=" + project + "/config/hm/commands\n" +
		"CUSTOM_PROPERTIES_DIR=" + project + "/config/docker\n" +
		"MACHINE=mac\n" +
		"DOCKER_COMPOSE_FILE_MACHINE=docker-compose.dev.mac.yml\n" +
		"DOCKER_COMPOSE=docker compose -f docker-compose.yml -f docker-compose.dev.mac.yml\n" +
		"HM_WORKTREE_ENVIRONMENT=false\n" +
		"HM_PROFILE=full\n" +
		"HM_EXIT_PROJECT=4\n" +
		"GREEN=\n" +
		"COLOR_RESET=\n" +
		"HM_OUTPUT_FORMAT=text\n"
	if stdout != want {
		t.Fatalf("got\n%s\nwant\n%s", stdout, want)
	}
}

func TestTheColoursAreHandedOverAsTheShellHeldThem(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("SHOW", "GREEN COLOR_RESET")

	stdout, _, _ := runProject(t, project, "hello")

	if want := "args=\nGREEN=\\033[0;32m\nCOLOR_RESET=\\033[0m\n"; stdout != want {
		t.Fatalf("got %q, want %q", stdout, want)
	}
}

func TestAProjectCommandIsNotFoundFromASubdirectory(t *testing.T) {
	_, spawned, project := projectFixture(t)
	sub := filepath.Join(project, "app", "code")
	writeScript(t, filepath.Join(sub, ".keep"), "", 0o644)

	_, stderr, code := runProject(t, sub, "hello", "--no-json")

	if code != exitUsage || stderr != "\nCommand not found: hello\n\n  hm --help\n\n" || spawned.ran("hello") {
		t.Fatalf("= %d %q, want command_not_found as bin/run answers from a subdirectory", code, stderr)
	}
}

func TestTheDaemonIsCheckedBeforeTheScriptRuns(t *testing.T) {
	_, spawned, project := projectFixture(t)
	spawned.docker = 1

	_, stderr, code := runProject(t, project, "hello", "--json")

	want := `{
  "schema_version": 1,
  "command": "hello",
  "ok": false,
  "error": {
    "code": 3,
    "type": "docker_unavailable",
    "message": "Docker is not running",
    "hint": "Start Docker and try again"
  }
}
`
	if code != exitDocker || stderr != want || spawned.ran("hello.sh") {
		t.Fatalf("= %d\n%s\nwant\n%s", code, stderr, want)
	}
}

func TestAnInvalidComposeConfigurationRefusesTheScript(t *testing.T) {
	_, spawned, project := projectFixture(t)
	spawned.compose = 15
	spawned.composeError = "yaml: line 3: mapping values are not allowed\n"

	_, stderr, code := runProject(t, project, "hello", "--no-json")

	want := "\nThis directory is not a configured Hiberus Dockergento project, or its Docker configuration " +
		"is invalid: yaml: line 3: mapping values are not allowed\n\n  hm setup\n\n"
	if code != exitProject || stderr != want || spawned.ran("hello.sh") {
		t.Fatalf("= %d %q\nwant %q", code, stderr, want)
	}
}

func TestAValidatedConfigurationIsNotValidatedAgainUntilItChanges(t *testing.T) {
	_, spawned, project := projectFixture(t)

	runProject(t, project, "hello")
	spawned.calls = nil
	runProject(t, project, "hello")

	for _, call := range spawned.calls {
		if len(call.args) > 0 && call.args[len(call.args)-1] == "-q" {
			t.Fatalf("validated again with nothing changed: %v", spawned.calls)
		}
	}
}

func TestADirectoryThatNamesNoProjectIsRefusedFirst(t *testing.T) {
	_, spawned, _ := projectFixture(t)
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	project := projectWithCommand(t, filepath.Join(dir, "___"), `{}`)

	_, stderr, code := runProject(t, project, "hello", "--no-json")

	want := "\nNo project name can be derived from ___\n\n" +
		"  Rename the directory, or set COMPOSE_PROJECT_NAME in config/docker/properties.json\n\n"
	if code != exitProject || stderr != want || spawned.ran("docker") {
		t.Fatalf("= %d %q, want no_project_name before any docker check", code, stderr)
	}
}

func TestANameNobodyAnswersForIsNotFound(t *testing.T) {
	_, _, project := projectFixture(t)

	_, stderr, code := runProject(t, project, "nope", "--json")

	want := `{
  "schema_version": 1,
  "command": "nope",
  "ok": false,
  "error": {
    "code": 2,
    "type": "command_not_found",
    "message": "Command not found: nope",
    "hint": "hm --help"
  }
}
`
	if code != exitUsage || stderr != want {
		t.Fatalf("= %d\n%s\nwant\n%s", code, stderr, want)
	}
}

func TestTheRefusalIsPaintedLikeTheShells(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	_, stderr, _ := runProject(t, project, "nope", "--no-json")

	want := "\033[0;31m\nCommand not found: nope\n\033[0m\033[0m\n  \033[0;33mhm --help\033[0m\033[0m\n\n"
	if stderr != want {
		t.Fatalf("got %q\nwant %q", stderr, want)
	}
}

// A key of the declarations that is not a command — the metadata starting with "_" — is not
// found like any other unknown name. There is no shell implementation left for it to have moved
// out of.
func TestADeclaredNameWithNoCommandIsNotFound(t *testing.T) {
	_, _, project := projectFixture(t)

	_, stderr, code := runProject(t, project, "_groups", "--no-json")

	if code != exitUsage || stderr != "\nCommand not found: _groups\n\n  hm --help\n\n" {
		t.Fatalf("= %d %q, want command_not_found", code, stderr)
	}
}

// The script is not told where a shell implementation would live: there is none.
func TestTheScriptIsNotPointedAtAShellTree(t *testing.T) {
	_, _, project := projectFixture(t)
	t.Setenv("SHOW", "HELPERS_DIR TASKS_DIR COMPONENTS_DIR")

	stdout, _, _ := runProject(t, project, "hello", "--no-json")

	if want := "args=\nHELPERS_DIR=\nTASKS_DIR=\nCOMPONENTS_DIR=\n"; stdout != want {
		t.Fatalf("got %q\nwant %q", stdout, want)
	}
}

func TestARoutedNameWinsOverAProjectScript(t *testing.T) {
	fake, spawned, project := projectFixture(t)
	writeScript(t, filepath.Join(project, "config", "hm", "commands", "list.sh"), "#!/bin/sh\necho mine\n", 0o755)

	stdout, _, _ := runProject(t, project, "list", "--json")

	if strings.Contains(stdout, "mine") || spawned.ran("list.sh") || len(fake.calls) == 0 {
		t.Fatalf("list ran the project's script (%q), want the binary's command", stdout)
	}
}

// From a worktree everything resolves against the main checkout: its commands, its root.
func gitWorktree(t *testing.T, project string) string {
	t.Helper()

	for _, args := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "base"},
		{"worktree", "add", "-q", filepath.Join(filepath.Dir(project), "rama"), "-b", "rama"},
	} {
		command := exec.Command("git", args...)
		command.Dir = project

		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	return filepath.Join(filepath.Dir(project), "rama")
}

func TestFromAWorktreeTheMainCheckoutsCommandsRun(t *testing.T) {
	_, _, project := projectFixture(t)
	worktree := gitWorktree(t, project)
	t.Setenv("SHOW", "HM_ROOT HM_IS_WORKTREE HM_WORKTREE HM_WORKTREE_ENVIRONMENT DOCKER_COMPOSE")

	stdout, stderr, code := runProject(t, filepath.Join(worktree, "config"), "hello")

	want := "args=\n" +
		"HM_ROOT=" + project + "\n" +
		"HM_IS_WORKTREE=true\n" +
		"HM_WORKTREE=rama\n" +
		"HM_WORKTREE_ENVIRONMENT=false\n" +
		"DOCKER_COMPOSE=docker compose --project-directory " + project + " -f " + project +
		"/docker-compose.yml -f " + project + "/docker-compose.dev.mac.yml\n"
	if code != 0 || stdout != want {
		t.Fatalf("= %d %q\ngot\n%s\nwant\n%s", code, stderr, stdout, want)
	}
}

func TestARegisteredWorktreeRunsInItsOwnEnvironment(t *testing.T) {
	fake, _, project := projectFixture(t)
	worktree := gitWorktree(t, project)
	t.Setenv("HM_WORKTREE_DIR", "/registry")
	t.Setenv("SHOW", "HM_ROOT HM_MAIN_ROOT HM_PARENT_PROJECT COMPOSE_PROJECT_NAME HM_PROJECT HM_PROFILE HM_AGENT "+
		"DOMAIN HM_WORKTREE_ENVIRONMENT HM_WORKTREE_OVERLAY CUSTOM_COMMANDS_DIR COMMANDS_DIR DOCKER_COMPOSE")

	fake.project = core.Project{Name: "tienda-rama", Root: worktree, InWorktree: true, Worktree: &core.Worktree{
		Name: "rama", Project: "tienda-rama", Domain: "rama.tienda.local", Path: worktree,
		ParentRoot: project, Branch: "rama", Parent: "tienda", Profile: "agent",
	}}

	stdout, stderr, code := runProject(t, worktree, "hello")

	want := "args=\n" +
		"HM_ROOT=" + worktree + "\n" +
		"HM_MAIN_ROOT=" + project + "\n" +
		"HM_PARENT_PROJECT=tienda\n" +
		"COMPOSE_PROJECT_NAME=tienda-rama\n" +
		"HM_PROJECT=tienda-rama\n" +
		"HM_PROFILE=agent\n" +
		"HM_AGENT=true\n" +
		"DOMAIN=rama.tienda.local\n" +
		"HM_WORKTREE_ENVIRONMENT=true\n" +
		"HM_WORKTREE_OVERLAY=/registry/tienda/rama.yml\n" +
		"CUSTOM_COMMANDS_DIR=" + worktree + "/config/hm/commands\n" +
		"COMMANDS_DIR=" + project + "/config/hm/commands\n" +
		"DOCKER_COMPOSE=docker compose --project-directory " + worktree + " -f " + worktree +
		"/docker-compose.yml -f " + worktree + "/docker-compose.dev.mac.yml -f /registry/tienda/rama.yml\n"
	if code != 0 || stdout != want {
		t.Fatalf("= %d %q\ngot\n%s\nwant\n%s", code, stderr, stdout, want)
	}
}

// bin/run executed the script from bash, and bash runs a file the kernel refuses for lacking a
// `#!` line (ENOEXEC) as a shell script of its own. A script written that way keeps working.
func TestAScriptWithoutAShebangRunsAsABashScript(t *testing.T) {
	_, _, project := projectFixture(t)
	writeScript(t, filepath.Join(project, "config", "hm", "commands", "bare.sh"), `printf 'args=%s\n' "$*"
printf 'zero=%s\n' "$(basename "$0")"
printf 'shell=%s\n' "${BASH_VERSION:+bash}"
printf 'project=%s\n' "$COMPOSE_PROJECT_NAME"
printf 'to stderr\n' >&2
exit 5
`, 0o755)

	stdout, stderr, code := runProject(t, project, "bare", "uno", "dos tres")

	want := "args=uno dos tres\nzero=bare.sh\nshell=bash\nproject=tienda\n"
	if code != 5 || stdout != want || stderr != "to stderr\n" {
		t.Fatalf("= %d %q %q, want 5 %q %q", code, stdout, stderr, want, "to stderr\n")
	}
}
