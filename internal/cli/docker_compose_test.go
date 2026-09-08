package cli

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What docker-compose asks of the engine: a project resolved first, then exactly one request to
// run Compose for that directory, with the arguments passed through verbatim. It is transparent —
// no envelope, the subcommand's own exit code comes back unchanged.
//

func TestWhatDockerComposeAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	args := []string{"config", "--format", "json"}

	code := dockerCompose(args, io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("docker-compose %v = %d, want exitOK", args, code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Compose", Dir: cwd, Command: args},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestComposeExitCodeIsPassedThrough(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {status: 17}}

	code := dockerCompose([]string{"up", "-d"}, io.Discard, io.Discard, false)

	if code != 17 {
		t.Fatalf("docker-compose with a non-zero exit = %d, want the runner's own 17, not an envelope", code)
	}
}

func TestDockerComposeOutsideAProjectIsRefused(t *testing.T) {
	fake := answering(t) // fake.project stays at its zero value: no project
	cwd := here()

	code := dockerCompose([]string{"config"}, io.Discard, io.Discard, false)

	if code != exitProject {
		t.Fatalf("docker-compose outside a project = %d, want exitProject", code)
	}

	want := []call{{Method: "Resolve", Dir: cwd}}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestAMissingComposeBinaryIsRefused(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "compose_missing",
		Code:    exitDocker,
		Message: "No Compose binary is installed",
		Hint:    "Install Docker or Docker Compose",
	}}}

	code := dockerCompose([]string{"config"}, io.Discard, io.Discard, false)

	if code != exitDocker {
		t.Fatalf("docker-compose with no binary = %d, want exitDocker", code)
	}
}
