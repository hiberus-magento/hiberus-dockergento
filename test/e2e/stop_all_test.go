package e2e_test

import (
	"os"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/test/e2e"
)

//
// docker-stop-all is machine-wide: it reaches every container on the machine, not only this
// project's. An automated test cannot know the machine is otherwise idle, so it never assumes
// it — the rule this suite follows is that it asserts only on containers it created itself, and
// it never runs the confirmed stop unless somebody explicitly asked for it.
//
// Enabling HM_E2E_STOP_ALL=1 makes TestStopAllConfirmedStopsThem run the real command, and the
// real command stops every running container on the machine it runs on, including whatever else
// happens to be up. It must never be set on a shared or development daemon: it belongs on a
// throwaway machine, and it is the automated form of this change's own manual verification.
//

func stoppable(t *testing.T) (*e2e.Session, e2e.Project) {
	t.Helper()

	session := e2e.New(t)
	project := e2e.NewProject(t, "hm-e2e-stop-all", e2e.Definition{}).Committed(t)

	e2e.Up(t, session, project)

	return session, project
}

// A worktree with no registration of its own resolves to the main checkout, and docker-stop-all
// is one of the commands that recreate or destroy an environment — refused there the same as
// start, stop and restart.
func TestStopAllRefusedFromAnUnregisteredWorktree(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := stoppable(t)

	worktree := t.TempDir() + "/unregistered"
	git(t, project.Root, "worktree", "add", "-q", "--detach", worktree)

	result := session.Run(t, worktree, "docker-stop-all")

	if result.Code != 6 {
		t.Fatalf("docker-stop-all from an unregistered worktree = %d, want 6\n%s", result.Code, result.Output())
	}

	if e2e.Compose(t, session, project, "ps", "-q", "phpfpm") == "" {
		t.Fatal("the project's own container is not running any more, want the refusal to have stopped nothing")
	}
}

// Answering no stops nothing — safe against whatever else the machine happens to have up, because
// nothing but y/Y ever stops anything.
func TestStopAllAnsweredNoStopsNothing(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	session, project := stoppable(t)

	result := session.RunWithInput(t, "n\n", project.Root, "--no-json", "docker-stop-all")

	if result.Code != 0 {
		t.Fatalf("docker-stop-all answered no = %d, want 0\n%s", result.Code, result.Output())
	}

	if !strings.Contains(result.Output(), "Nothing was stopped.") {
		t.Fatalf("docker-stop-all answered no = %q, want it to say nothing was stopped", result.Output())
	}

	if e2e.Compose(t, session, project, "ps", "-q", "phpfpm") == "" {
		t.Fatal("the project's own container is not running any more, want a decline to have stopped nothing")
	}
}

// TestStopAllConfirmedStopsThem is the one scenario that actually stops something, so it is
// gated behind both NeedsDocker and an explicit opt-in: see the package comment above.
func TestStopAllConfirmedStopsThem(t *testing.T) {
	t.Parallel()
	e2e.NeedsDocker(t)

	if os.Getenv("HM_E2E_STOP_ALL") != "1" {
		t.Skip("skipping: stops every running container on the machine; " +
			"opt in with HM_E2E_STOP_ALL=1 on a throwaway machine only")
	}

	session, project := stoppable(t)

	result := session.RunWithInput(t, "y\n", project.Root, "--no-json", "docker-stop-all")

	if result.Code != 0 {
		t.Fatalf("docker-stop-all answered yes = %d, want 0\n%s", result.Code, result.Output())
	}

	if e2e.Compose(t, session, project, "ps", "-q", "phpfpm") != "" {
		t.Fatal("the project's own container is still running, want the confirmed stop to have stopped it")
	}
}
