package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Composer's macOS vendor dance, ported from composer.sh: on macOS the four subcommands that write
// the dependencies copy the host's vendor in, run Composer, and copy the whole workdir back out
// over the host's. What is pinned here is the order of the steps, that a failed one stops the
// rest, and which invocations take that path at all.
//

// onPlatform pins the platform the dance is decided on for the rest of this test.
func onPlatform(t *testing.T, platform string) {
	t.Helper()

	original := goos
	goos = platform
	t.Cleanup(func() { goos = original })
}

// composerShop is a project with an environment of its own, the ordinary case.
func composerShop() core.Project {
	return core.Project{Name: "tienda", Root: "/code/shop", MagentoDir: "."}
}

// workingDirectory is what here() answers in a test that answering(t) moved to a temporary
// directory: the directory the engine calls of the dance are made about.
func workingDirectory(t *testing.T) string {
	t.Helper()

	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	return directory
}

// dance is everything the dance asks of the engine, in its order, for the given Composer arguments.
func dance(dir string, project core.Project, files []string, args ...string) []call {
	return []call{
		{Method: "Resolve", Dir: dir},
		{Method: "Restart", Dir: dir, Services: []string{phpService}},
		{Method: "CopyInto", Dir: dir, Paths: []string{"vendor"}},
		{Method: "Exec", Dir: project.Root, Service: phpService,
			Command: append([]string{"composer"}, args...), Options: terminalOptions("")},
		{Method: "DefaultMagentoFiles"},
		{Method: "CopyInto", Dir: dir, Paths: files},
		{Method: "Stop", Dir: dir, Services: []string{phpService}},
		{Method: "ClearVendor", Dir: dir},
		{Method: "CopyFrom", Dir: dir, Paths: []string{"."}},
		{Method: "Start", Dir: dir, Services: []string{phpService}},
	}
}

func TestComposerOnMacOSMirrorsTheVendorStepByStep(t *testing.T) {
	onPlatform(t, "darwin")

	for _, args := range [][]string{
		{"install"},
		{"update"},
		{"remove", "vendor/pkg"},
		{"require", "vendor/pkg", "--dev"},
	} {
		t.Run(args[0], func(t *testing.T) {
			fake := answering(t)
			fake.project = composerShop()
			files := []string{".htaccess", "bin", "pub"}
			fake.outcomes = make([]outcome, 5)
			fake.outcomes[4] = outcome{files: files}

			if code := composer(args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("composer %v = %d, want %d", args, code, exitOK)
			}

			want := dance(workingDirectory(t), fake.project, files, args...)
			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAFailedStepOfTheDanceStopsTheRest(t *testing.T) {
	onPlatform(t, "darwin")

	t.Run("the restart", func(t *testing.T) {
		fake := answering(t)
		fake.project = composerShop()
		fake.outcomes = []outcome{{}, {err: core.Refusal{Kind: "blocked", Code: 6, Message: "no"}}}

		if code := composer([]string{"install"}, io.Discard, io.Discard, false); code != 6 {
			t.Fatalf("composer install = %d, want the restart's 6", code)
		}

		want := dance(workingDirectory(t), fake.project, nil, "install")[:2]
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("composer itself", func(t *testing.T) {
		fake := answering(t)
		fake.project = composerShop()
		fake.outcomes = []outcome{{}, {}, {}, {status: 17}}

		if code := composer([]string{"install"}, io.Discard, io.Discard, false); code != 17 {
			t.Fatalf("composer install = %d, want composer's own 17", code)
		}

		// Above all, the host's vendor is not cleared for a tree Composer never finished
		want := dance(workingDirectory(t), fake.project, nil, "install")[:4]
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("the copy back out", func(t *testing.T) {
		fake := answering(t)
		fake.project = composerShop()
		fake.outcomes = make([]outcome, 9)
		fake.outcomes[8] = outcome{err: errors.New("copy failed")}

		if code := composer([]string{"install"}, io.Discard, io.Discard, false); code != exitDocker {
			t.Fatalf("composer install = %d, want %d", code, exitDocker)
		}

		// phpfpm is left stopped, as set -e left it in composer.sh
		want := dance(workingDirectory(t), fake.project, nil, "install")[:9]
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})
}

func TestComposerRunsPlainlyWhereThereIsNoDance(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		args     []string
	}{
		{"show on macOS", "darwin", []string{"show"}},
		{"no subcommand on macOS", "darwin", nil},
		{"install on Linux", "linux", []string{"install"}},
		{"require on Linux", "linux", []string{"require", "vendor/pkg"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			onPlatform(t, tt.platform)
			fake := answering(t)
			fake.project = composerShop()

			if code := composer(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("composer %v = %d, want %d", tt.args, code, exitOK)
			}

			want := []call{
				{Method: "Resolve", Dir: workingDirectory(t)},
				{Method: "Exec", Dir: fake.project.Root, Service: phpService,
					Command: append([]string{"composer"}, tt.args...), Options: terminalOptions("")},
			}
			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTheDanceIsRefusedFromAWorktreeOfTheMainCheckout(t *testing.T) {
	onPlatform(t, "darwin")

	// A worktree with no environment of its own resolves against the main checkout
	borrowed := core.Project{Name: "tienda", Root: "/code/shop", MagentoDir: ".", InWorktree: true}

	t.Run("refused before anything is restarted", func(t *testing.T) {
		t.Setenv("HM_FORCE", "")
		fake := answering(t)
		fake.project = borrowed

		var stderr bytes.Buffer
		if code := composer([]string{"install"}, io.Discard, &stderr, false); code != exitBlocked {
			t.Fatalf("composer install = %d, want %d", code, exitBlocked)
		}

		for _, wanted := range []string{
			"'composer install' is refused from a worktree on macOS: it would restart phpfpm and rewrite the dependencies of /code/shop",
			"Run it from /code/shop, or repeat as hm --force composer install",
		} {
			if !strings.Contains(stderr.String(), wanted) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), wanted)
			}
		}

		if diff := cmp.Diff([]call{{Method: "Resolve", Dir: workingDirectory(t)}}, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("its kind, and every argument in the way out", func(t *testing.T) {
		t.Setenv("HM_FORCE", "")
		fake := answering(t)
		fake.project = borrowed

		var stderr bytes.Buffer
		composer([]string{"require", "vendor/pkg", "--dev"}, io.Discard, &stderr, true)

		for _, wanted := range []string{`"blocked_in_worktree"`, "hm --force composer require vendor/pkg --dev"} {
			if !strings.Contains(stderr.String(), wanted) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), wanted)
			}
		}
	})

	t.Run("forced, the dance runs", func(t *testing.T) {
		t.Setenv("HM_FORCE", "1")
		fake := answering(t)
		fake.project = borrowed

		if code := composer([]string{"install"}, io.Discard, io.Discard, false); code != exitOK {
			t.Fatalf("composer install = %d, want %d", code, exitOK)
		}

		want := dance(workingDirectory(t), fake.project, nil, "install")
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("a subcommand that writes no dependencies is not refused", func(t *testing.T) {
		t.Setenv("HM_FORCE", "")
		fake := answering(t)
		fake.project = borrowed

		if code := composer([]string{"show"}, io.Discard, io.Discard, false); code != exitOK {
			t.Fatalf("composer show = %d, want %d", code, exitOK)
		}
	})
}

func TestTheEarlierRefusalsStillComeFirst(t *testing.T) {
	onPlatform(t, "darwin")

	t.Run("a worktree that reads the main checkout's dependencies", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "tienda-x", Root: "/code/wt", MagentoDir: ".", InWorktree: true,
			Worktree: &core.Worktree{SharedVendor: true, ParentRoot: "/code/shop"}}

		var stderr bytes.Buffer
		if code := composer([]string{"install"}, io.Discard, &stderr, true); code != exitBlocked {
			t.Fatalf("composer install = %d, want %d", code, exitBlocked)
		}

		if !strings.Contains(stderr.String(), `"shared_dependencies"`) {
			t.Fatalf("stderr = %q, want shared_dependencies", stderr.String())
		}

		if len(fake.calls) != 1 {
			t.Fatalf("asked of the engine %v, want only Resolve", fake.calls)
		}
	})

	t.Run("create-project", func(t *testing.T) {
		fake := answering(t)
		fake.project = composerShop()

		if code := composer([]string{"create-project", "x"}, io.Discard, io.Discard, false); code != exitUsage {
			t.Fatalf("composer create-project = %d, want %d", code, exitUsage)
		}

		if len(fake.calls) != 0 {
			t.Fatalf("asked of the engine %v, want nothing", fake.calls)
		}
	})
}

func TestTheRouterAnswersEveryComposerInvocation(t *testing.T) {
	onPlatform(t, "darwin")
	fake := answering(t)
	fake.project = composerShop()

	Run([]string{"composer", "install"}, io.Discard, io.Discard)

	// Falling through to the project-command lookup would reach the real engine and leave the fake unasked
	if len(fake.calls) < 2 || fake.calls[1].Method != "Restart" {
		t.Fatalf("composer install asked the engine %v, want the dance through the handler", fake.calls)
	}
}
