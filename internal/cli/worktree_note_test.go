package cli

import (
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The note a command that works on the code prints from a worktree with no environment of its
// own: the containers it reaches are the main checkout's, so they serve the main checkout's code,
// not the code the person is looking at.
//
// The source of truth is the shell implementation: bin/run's validate_worktree (:261-262) prints
// it, on stderr, for the names hm_operates_on_code lists (console/helpers/worktree.sh:99-108) —
// magento, composer, npm, grunt, exec, bash, test-unit, test-integration and n98-magerun — once a
// worktree is known not to have an environment of its own. grunt is still the shell's, so it is
// not here; every other name on that list is answered by this binary, and is.
//
// Through Run and not the handlers, because what the shell list names is commands: bash reaches
// the container through exec's handler, and a test of the handlers would not notice a command
// routed somewhere that does not print it.
//

const mainCheckout = "/code/shop"

// borrowed is a git worktree with no environment of its own, which resolves to the main
// checkout: its Root is the main checkout's, which is what the note names.
var borrowed = core.Project{Name: "shop", Root: mainCheckout, InWorktree: true}

var codeCommands = map[string][]string{
	"magento":          {"magento", "cache:clean"},
	"composer":         {"composer", "show"},
	"npm":              {"npm", "run", "build"},
	"n98-magerun":      {"n98-magerun", "sys:info"},
	"test-unit":        {"test-unit"},
	"test-integration": {"test-integration"},
	"exec":             {"exec", "ls"},
	"bash":             {"bash"},
}

const note = "Note: the containers serve the code of " + mainCheckout + ", not this worktree."

func runCapturing(t *testing.T, project core.Project, args []string) (stdout, stderr string) {
	t.Helper()

	fake := answering(t)
	fake.project = project

	out, errs := &strings.Builder{}, &strings.Builder{}

	if code := Run(args, out, errs); code != exitOK {
		t.Fatalf("%v = %d, want exitOK; stderr:\n%s", args, code, errs.String())
	}

	if len(fake.calls) == 0 {
		t.Fatalf("%v asked the fake engine nothing: it fell through to the legacy runner", args)
	}

	return out.String(), errs.String()
}

func TestACodeCommandFromABorrowingWorktreeSaysWhoseCodeItIs(t *testing.T) {
	for name, args := range codeCommands {
		t.Run(name, func(t *testing.T) {
			stdout, stderr := runCapturing(t, borrowed, args)

			if strings.Count(stderr, note) != 1 {
				t.Errorf("%v stderr = %q, want the note exactly once:\n%s", args, stderr, note)
			}

			// stdout is the container's, and something like `hm magento module:status | grep`
			// reads it: the note must never land there
			if strings.Contains(stdout, "Note:") {
				t.Errorf("%v stdout = %q, want the note kept out of it", args, stdout)
			}
		})
	}
}

func TestTheNoteIsNotPrintedWhereItWouldNotBeTrue(t *testing.T) {
	ownEnvironment := core.Project{Name: "shop-feature", Root: "/code/shop-feature", InWorktree: true,
		Worktree: &core.Worktree{Name: "feature", Path: "/code/shop-feature", ParentRoot: mainCheckout}}

	cases := []struct {
		name    string
		project core.Project
		args    []string
	}{
		// The containers are this checkout's own, serving this checkout's code
		{"magento from the main checkout", core.Project{Name: "shop", Root: mainCheckout},
			[]string{"magento", "cache:clean"}},
		{"exec from the main checkout", core.Project{Name: "shop", Root: mainCheckout},
			[]string{"exec", "ls"}},

		// A worktree with an environment of its own serves its own code, which the shell skips
		// before the note is reached (bin/run:237-239)
		{"magento from a worktree with its own environment", ownEnvironment,
			[]string{"magento", "cache:clean"}},

		// purge reaches the same container the same way, but it is not on the shell's list
		{"purge from a borrowing worktree", borrowed, []string{"purge"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := runCapturing(t, tt.project, tt.args)

			if strings.Contains(stderr, "serve the code") {
				t.Errorf("%v stderr = %q, want no note", tt.args, stderr)
			}
		})
	}
}
