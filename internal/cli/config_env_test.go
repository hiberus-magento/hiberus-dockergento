package cli

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func TestWhatConfigEnvAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test"}
	cwd := here()

	if code := configEnv(nil, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("config-env = %d, want exitOK", code)
	}

	want := []call{{Method: "Resolve", Dir: cwd}, {Method: "ConfigureForDevelopment", Dir: cwd}}
	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestConfigEnvRefusesAnArgument: it takes none, and one that would silently do nothing is refused.
func TestConfigEnvRefusesAnArgument(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	if code := configEnv([]string{"--store=1"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("config-env --store=1 = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestAConfigEnvRefusalKeepsItsCode: a project with no domain is refused by the engine with 2.
func TestAConfigEnvRefusalKeepsItsCode(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{Kind: "no_domain", Code: 2, Message: "no domain"}}}

	if code := configEnv(nil, io.Discard, io.Discard, true); code != 2 {
		t.Fatalf("config-env with no domain = %d, want 2", code)
	}
}

// TestTheRouterAnswersForConfigEnv: it reaches its handler instead of the shell implementation.
func TestTheRouterAnswersForConfigEnv(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test"}

	Run([]string{"config-env"}, io.Discard, io.Discard)

	if len(fake.calls) == 0 {
		t.Fatal("config-env fell through to the shell implementation")
	}
}
