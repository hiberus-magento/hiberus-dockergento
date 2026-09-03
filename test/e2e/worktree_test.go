package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/test/e2e"
)

//
// Branch environments: a git worktree with a Docker environment of its own.
//
// The registration lives outside the checkout, in a database this tool keeps. A worktree shares
// the repository's tracked files, and `config/docker/properties.json` is one of them: writing the
// worktree's project name there would put it in somebody's commit and rename the main environment
// at the same time.
//
// There is one implementation of this now — the shell entry point delegates to the binary, which
// is what let the registry stop being a directory of small files — so nothing here compares two
// answers. What it asserts is what the command does.
//

// branches is a project that can have branch environments: committed, so git can make a worktree
// of it, and with somewhere for the registry to live.
func branches(t *testing.T) (*e2e.Session, e2e.Project) {
	t.Helper()

	session := e2e.New(t)

	project := e2e.NewProject(t, "hm-e2e-ramas", e2e.Definition{
		Services: `  phpfpm:
    image: alpine:latest
    command: sh -c "sleep 600"
    stop_grace_period: 1s
`,
		Properties: map[string]string{"MAGENTO_DIR": "./src", "DOMAIN": "ramas.invalid"},
	})

	project.Write(t, "src/.keep", "")
	project.Committed(t)

	return session, project
}

// registers writes a registration in the shape earlier versions wrote it, and makes the git
// worktree it describes.
//
// That is the point of doing it this way: what those versions wrote, this one has to read.
func registers(t *testing.T, session *e2e.Session, project e2e.Project, name string) string {
	t.Helper()

	worktree := filepath.Join(t.TempDir(), "rama-"+name)

	git(t, project.Root, "worktree", "add", "-q", worktree, "-b", name)

	directory := filepath.Join(session.Home, ".hm", "worktrees", project.Name)

	session.Writes(t, filepath.Join(directory, name+".json"), fmt.Sprintf(
		`{"path":%q,"branch":%q,"profile":"agent","domain":"%s.ramas.invalid",`+
			`"project":"%s-%s","created":"2026-09-02 10:00","vendor":"own"}`,
		worktree, name, name, project.Name, name))

	session.Writes(t, filepath.Join(directory, name+".yml"), "services: {}\n")

	return worktree
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v = %v\n%s", args, err, output)
	}

	return strings.TrimSpace(string(output))
}

// What earlier versions wrote is drained on the way in rather than abandoned: a machine with
// branch environments in it keeps them.
func TestARegistrationAnEarlierVersionWroteIsStillFound(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	registers(t, session, project, "azul")

	listed := session.Run(t, project.Root, "worktree", "list", "--json")

	if name := e2e.String(t, listed.Stdout, "data.worktrees.0.name"); name != "azul" {
		t.Fatalf("worktree list = %q, want the registration azul in it", listed.Stdout)
	}
}

// The entry point people type is still `hm`, and it has to arrive at the same place: the shell
// implementation of this command is a delegator now, which is what makes one registry possible.
func TestTheShellEntryPointReachesTheSameAnswer(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	registers(t, session, project, "azul")

	fromBinary := session.Run(t, project.Root, "worktree", "list", "--json")
	fromShell := session.RunShell(t, project.Root, "worktree", "list", "--json")

	if difference := e2e.Difference(t, fromShell.Stdout, fromBinary.Stdout); difference != "" {
		t.Errorf("bin/run worktree list differs from the binary's answer (-want +got):\n%s", difference)
	}
}

// A directory somebody deleted by hand leaves a registration behind. Saying "stopped" about it
// would send them looking for containers that are not the problem.
func TestAWorktreeWhoseDirectoryIsGoneIsReportedAsMissing(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	worktree := registers(t, session, project, "azul")

	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}

	listed := session.Run(t, project.Root, "worktree", "list", "--json")

	if state := e2e.String(t, listed.Stdout, "data.worktrees.0.state"); state != "missing" {
		t.Errorf("state of a worktree whose directory is gone = %q, want %q", state, "missing")
	}
}

// Removing has to clear both: the row, and the file earlier versions wrote. Leaving the file would
// bring the environment back on the next listing, because that file is read on the way in.
func TestRemovingClearsTheRegistrationAndWhatTheOldDirectoryHeld(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	worktree := registers(t, session, project, "azul")

	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}

	git(t, project.Root, "worktree", "prune")
	session.Run(t, project.Root, "--yes", "--force", "worktree", "remove", "azul")

	listed := session.Run(t, project.Root, "worktree", "list", "--json")

	if worktrees, _ := e2e.Field(t, listed.Stdout, "data.worktrees").([]any); len(worktrees) != 0 {
		t.Errorf("worktree list after removing = %q, want nothing registered", listed.Stdout)
	}

	legacy := filepath.Join(session.Home, ".hm", "worktrees", project.Name, "azul.json")
	if _, err := os.Stat(legacy); err == nil {
		t.Errorf("%s exists, want what earlier versions wrote gone with the row", legacy)
	}
}

func TestWithNoneRegisteredItSaysSoAndHowToMakeOne(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)

	listed := session.Run(t, project.Root, "--no-json", "worktree", "list")

	if !strings.Contains(listed.Output(), "No branch environments") {
		t.Errorf("worktree list with none registered = %q, want it to say so", listed.Output())
	}

	if !strings.Contains(listed.Output(), "worktree add") {
		t.Errorf("worktree list with none registered = %q, want it to say how to make one", listed.Output())
	}
}

func TestRemovingSomethingThatIsNotThereIsARefusal(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)

	cases := map[string][]string{
		"a name nobody registered": {"worktree", "remove", "noexiste"},
		"no name at all":           {"worktree", "remove"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := session.Run(t, project.Root, args...)

			if got.Code != 2 {
				t.Errorf("%v = %d, want 2\n%s", args, got.Code, got.Output())
			}

			if hint := e2e.String(t, got.Stderr, "error.hint"); !strings.Contains(hint, "worktree list") {
				t.Errorf("hint = %q, want it to name `worktree list`", hint)
			}
		})
	}
}

// Containers and databases can be rebuilt in seconds; uncommitted code cannot be rebuilt at all,
// which is why git's own refusal is repeated rather than worked around.
func TestAWorktreeWithUncommittedChangesIsRefused(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	worktree := registers(t, session, project, "verde")

	if err := os.WriteFile(filepath.Join(worktree, "pendiente.txt"), []byte("sin guardar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	git(t, worktree, "add", "pendiente.txt")

	result := session.Run(t, project.Root, "worktree", "remove", "verde")

	if result.Code != 6 {
		t.Fatalf("worktree remove verde = %d, want 6\n%s", result.Code, result.Output())
	}

	if kind := e2e.String(t, result.Stderr, "error.type"); kind != "uncommitted_changes" {
		t.Errorf("error type = %q, want %q", kind, "uncommitted_changes")
	}
}

// Removing a branch environment destroys its data, so it asks — and an answer that is not yes
// leaves it alone.
func TestRemovingAsksAndTakesNoForAnAnswer(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	registers(t, session, project, "verde")

	before := session.Run(t, project.Root, "worktree", "list", "--json")
	registered, _ := e2e.Field(t, before.Stdout, "data.worktrees").([]any)

	session.RunWithInput(t, "otra-cosa\n", project.Root, "--no-json", "worktree", "remove", "verde")

	after := session.Run(t, project.Root, "worktree", "list", "--json")
	still, _ := e2e.Field(t, after.Stdout, "data.worktrees").([]any)

	if len(still) != len(registered) {
		t.Errorf("registered after answering something else = %d, want %d", len(still), len(registered))
	}
}

func TestRemovingTakesTheRegistrationTheOverlayAndTheWorktree(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)
	worktree := registers(t, session, project, "verde")

	result := session.Run(t, project.Root, "--yes", "--no-json", "worktree", "remove", "verde")

	if !strings.Contains(result.Output(), "Removed") {
		t.Errorf("worktree remove verde said %q, want it to say it was removed", result.Output())
	}

	listed := session.Run(t, project.Root, "worktree", "list", "--json")
	if strings.Contains(listed.Stdout, "verde") {
		t.Errorf("worktree list = %q, want verde gone from it", listed.Stdout)
	}

	overlay := filepath.Join(session.Home, ".hm", "worktrees", project.Name, "verde.yml")
	if _, err := os.Stat(overlay); err == nil {
		t.Errorf("%s exists, want the overlay gone with it", overlay)
	}

	if _, err := os.Stat(worktree); err == nil {
		t.Errorf("%s exists, want the worktree gone with it", worktree)
	}
}

func TestAddingWithNothingUsableIsARefusal(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)

	cases := map[string][]string{
		"no branch at all":          {"worktree", "add"},
		"a profile that is not one": {"worktree", "add", "--profile=inventado", "rama"},
		"an option nobody declared": {"worktree", "add", "--tonteria"},
		"two branches":              {"worktree", "add", "una", "otra"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := session.Run(t, project.Root, args...); got.Code != 2 {
				t.Errorf("%v = %d, want 2\n%s", args, got.Code, got.Output())
			}
		})
	}
}

// A branch environment answers on its own address, and there is nothing to answer with unless the
// project is routed through the global proxy.
func TestWithoutTheProxyAddingIsRefused(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)

	result := session.Run(t, project.Root, "worktree", "add", "sinproxy")

	if result.Code != 6 {
		t.Fatalf("worktree remove verde = %d, want 6\n%s", result.Code, result.Output())
	}

	if kind := e2e.String(t, result.Stderr, "error.type"); kind != "proxy_required" {
		t.Errorf("error type = %q, want %q", kind, "uncommitted_changes")
	}
}

// Making one.
//
// A branch environment is reached by name, which needs the global proxy: without it every one of
// them would publish its own ports, which is the collision the proxy was built to end. So the
// project has to say it uses one.
func TestAddingABranchEnvironment(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := branches(t)

	project.Write(t, "config/docker/properties.json", fmt.Sprintf(
		`{"MAGENTO_DIR": "./src", "DOMAIN": "ramas.invalid", "COMPOSE_PROJECT_NAME": %q, "USE_PROXY": "true"}`,
		project.Name))

	if got := session.Run(t, project.Root, "worktree", "add", "rojo", "--no-start"); got.Code != 0 {
		t.Fatalf("worktree add rojo --no-start = %d, want 0\n%s", got.Code, got.Output())
	}

	registry := filepath.Join(session.Home, ".hm", "worktrees", project.Name)

	t.Run("the registration is recorded", func(t *testing.T) {
		listed := session.Run(t, project.Root, "worktree", "list", "--json")

		if got := e2e.String(t, listed.Stdout, "data.worktrees.0.name"); got != "rojo" {
			t.Errorf("worktree list = %q, want rojo in it", listed.Stdout)
		}
	})

	//
	// The registration is in the database now, and the overlay is still a file: a compose file is
	// something Docker reads, and one in a database is one nothing can load.
	//
	t.Run("and not as a file any more", func(t *testing.T) {
		if _, err := os.Stat(filepath.Join(registry, "rojo.json")); err == nil {
			t.Errorf("%s exists, want the registration in the database instead", registry+"/rojo.json")
		}
	})

	//
	// The overlay is where a mistake is invisible until something is running: a service written
	// twice is not a merge — the last one wins and the earlier block disappears without a word.
	//
	t.Run("the overlay is written where compose can load it", func(t *testing.T) {
		overlay, err := os.ReadFile(filepath.Join(registry, "rojo.yml"))
		if err != nil {
			t.Fatalf("reading the overlay = %v, want it written", err)
		}

		for _, want := range []string{project.Name + "-rojo", "services:"} {
			if !strings.Contains(string(overlay), want) {
				t.Errorf("overlay = %q, want %q in it", overlay, want)
			}
		}
	})

	t.Run("and the worktree is on disk, on its own branch", func(t *testing.T) {
		worktree := filepath.Join(filepath.Dir(project.Root), project.Name+"-worktrees", "rojo")

		if _, err := os.Stat(worktree); err != nil {
			t.Fatalf("stat %s = %v, want the worktree there", worktree, err)
		}

		if got := git(t, worktree, "rev-parse", "--abbrev-ref", "HEAD"); got != "rojo" {
			t.Errorf("branch checked out = %q, want %q", got, "rojo")
		}
	})

	//
	// Two environments answering to one name is two sets of containers where one was meant, and
	// whichever answers is whichever Docker picked.
	//
	t.Run("a name that is already taken is refused", func(t *testing.T) {
		got := session.Run(t, project.Root, "worktree", "add", "rojo")

		if got.Code != 2 {
			t.Fatalf("worktree add rojo (again) = %d, want 2\n%s", got.Code, got.Output())
		}

		if kind := e2e.String(t, got.Stderr, "error.type"); kind != "already_registered" {
			t.Errorf("error type = %q, want %q", kind, "already_registered")
		}
	})

	t.Cleanup(func() {
		session.Run(t, project.Root, "--yes", "--force", "worktree", "remove", "rojo")
	})
}
