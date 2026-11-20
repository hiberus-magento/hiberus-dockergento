package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What create-project asks of the engine. The handler reads the whole instruction with
// core.ParseCreateProject — pinned in core/create_project_test.go — asks for one CreateProject, and
// then resolves the project it made to say where it answers. What creating a project is — the
// questions, the compose files, the steps — is app.Setup's, and is tested there.
//

func TestWhatCreateProjectAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Domain: "shop.local", MagentoDir: "src"}
	cwd := here()

	args := []string{"-p", "shop", "-e", "enterprise", "-v", "2.4.7", "-r", "src"}
	if code := createProject(args, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("create-project %v = %d, want exitOK", args, code)
	}

	// answering(t) sets HM_NON_INTERACTIVE, so nothing may be asked
	want := []call{
		{Method: "CreateProject", Dir: cwd, Interactive: false, CreateProjectOptions: core.CreateProjectOptions{
			ProjectName: "shop", Domain: "shop.local", Edition: "enterprise", Version: "2.4.7", Root: "src",
		}},
		{Method: "Resolve", Dir: cwd},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestCreateProjectMayAskWithSomebodyThere: with nobody saying otherwise, it may ask its questions.
func TestCreateProjectMayAskWithSomebodyThere(t *testing.T) {
	fake := answering(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	fake.project = core.Project{Name: "shop", Domain: "shop.local"}

	if code := createProject(nil, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("create-project = %d, want exitOK", code)
	}

	if got := fake.calls[0]; got.Method != "CreateProject" || !got.Interactive {
		t.Fatalf("first call = %+v, want an interactive CreateProject", got)
	}
}

// TestCreateProjectRefusesAnInstructionItCannotRead: a malformed instruction asks nothing.
func TestCreateProjectRefusesAnInstructionItCannotRead(t *testing.T) {
	for _, args := range [][]string{{"--nonsense"}, {"stray"}, {"-e", "open-source"}} {
		fake := answering(t)

		if code := createProject(args, io.Discard, io.Discard, false); code != exitUsage {
			t.Fatalf("create-project %v = %d, want exitUsage", args, code)
		}

		if len(fake.calls) != 0 {
			t.Errorf("create-project %v asked the engine %v, want nothing", args, fake.calls)
		}
	}
}

// TestCreateProjectSaysWhereTheProjectAnswers: the address to open, from the project it made.
func TestCreateProjectSaysWhereTheProjectAnswers(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Domain: "shop.local"}

	var text bytes.Buffer
	if code := createProject([]string{"-u"}, &text, io.Discard, false); code != exitOK {
		t.Fatalf("create-project -u = %d, want exitOK", code)
	}

	if !strings.Contains(text.String(), "https://shop.local/") {
		t.Errorf("stdout = %q, want the address to open", text.String())
	}
}

// TestAFailedCreateProjectKeepsItsCode: an error from the engine stops there, with nothing resolved.
func TestAFailedCreateProjectKeepsItsCode(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "setup_failed", Code: 17, Message: "failed"}}}

	if code := createProject(nil, io.Discard, io.Discard, false); code != 17 {
		t.Fatalf("create-project with a failing engine = %d, want the refusal's own 17", code)
	}

	if len(fake.calls) != 1 {
		t.Errorf("asked the engine %v, want only the failed CreateProject", fake.calls)
	}
}

// TestTheRouterAnswersForCreateProject: the case line in run.go is what reaches the handler; without
// it the command falls through to the project-command lookup and the fake is asked nothing.
func TestTheRouterAnswersForCreateProject(t *testing.T) {
	fake := answering(t)

	Run([]string{"create-project", "-u"}, io.Discard, io.Discard)

	if len(fake.calls) == 0 || fake.calls[0].Method != "CreateProject" {
		t.Fatalf("create-project asked %v, want the router to reach the handler instead of falling "+
			"through to the project-command lookup", fake.calls)
	}
}
