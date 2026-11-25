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
// The Magento Cloud CLI, run in the php container. Both commands were one line of shell each:
// `cloud` handed its arguments to `bash -c "magento-cloud $@"`, joined into one string, and
// `cloud-login` printed where to get a token and ran the CLI's own login.
//

func TestCloudRunsTheMagentoCloudCLIInThePHPContainer(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	if code := cloud([]string{"environment:list", "-p", "abc"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("cloud = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: "/code/shop", Service: phpService,
			Command: []string{"bash", "-c", "magento-cloud environment:list -p abc"}, Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestCloudAnswersWithTheCLIsOwnStatus: what the CLI exits with is what the command exits with.
func TestCloudAnswersWithTheCLIsOwnStatus(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {status: 17}}

	if code := cloud(nil, io.Discard, io.Discard, false); code != 17 {
		t.Fatalf("cloud = %d, want the CLI's own 17", code)
	}
}

// TestCloudOutsideAProjectRunsNothing: the project is resolved first.
func TestCloudOutsideAProjectRunsNothing(t *testing.T) {
	fake := answering(t)

	if code := cloud(nil, io.Discard, io.Discard, false); code != exitProject {
		t.Fatalf("cloud outside a project = %d, want exitProject", code)
	}

	for _, c := range fake.calls {
		if c.Method == "Exec" {
			t.Fatalf("ran something outside a project: %v", fake.calls)
		}
	}
}

func TestCloudLoginSaysWhereTheTokenIsAndRunsTheCLIsLogin(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	var stdout bytes.Buffer
	if code := cloudLogin(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("cloud-login = %d, want exitOK", code)
	}

	if !strings.Contains(stdout.String(), "https://accounts.magento.cloud/user/api-tokens") {
		t.Errorf("stdout = %q, want where to generate the token", stdout.String())
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: "/code/shop", Service: phpService,
			Command: []string{"bash", "-c", "magento-cloud auth:api-token-login"}, Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestTheRouterAnswersForTheCloudCommands: both reach their handler instead of the shell.
func TestTheRouterAnswersForTheCloudCommands(t *testing.T) {
	for _, args := range [][]string{{"cloud", "list"}, {"cloud-login"}} {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		Run(args, io.Discard, io.Discard)

		if len(fake.calls) == 0 {
			t.Fatalf("%v fell through to the shell implementation", args)
		}
	}
}

// TestCloudLoginRefusesAnArgument: it takes none, and one that would silently do nothing is
// refused before anything runs, as config-env refuses them.
func TestCloudLoginRefusesAnArgument(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	t.Setenv("COMMAND_BIN_NAME", "hm")

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "cloud-login", "--token=abc"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("cloud-login --token=abc = %d, want exitUsage", code)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}

	assertError(t, stderr.String(), "invalid_argument", "cloud-login takes no arguments, and was given '--token=abc'", "hm cloud-login")

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestCloudLoginKeepsStdoutCleanUnderJSON: the sentence saying where the token is generated is
// for a person, so under --json it goes to stderr and stdout carries nothing of its own.
func TestCloudLoginKeepsStdoutCleanUnderJSON(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "cloud-login"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("cloud-login --json = %d, want exitOK", code)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}

	if !strings.Contains(stderr.String(), "https://accounts.magento.cloud/user/api-tokens") {
		t.Errorf("stderr = %q, want where to generate the token", stderr.String())
	}

	if len(fake.calls) != 2 || fake.calls[1].Method != "Exec" {
		t.Errorf("asked the engine %v, want Resolve and the login", fake.calls)
	}
}
