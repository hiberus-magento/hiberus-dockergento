// Package cli is a driving adapter: it turns a command line into a use case and a use case's
// answer into output. It is the only place in the Go tree that knows a terminal exists.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/fsprops"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/legacy"
	"github.com/hiberus-magento/hiberus-dockergento/internal/app"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// Version is stamped at build time. Empty means a build nobody released.
var Version = ""

// Run dispatches one invocation and returns the exit code.
//
// Everything the shell implementation still owns goes straight through to it, arguments
// untouched. That is the requirement of this stage: somebody installs the binary and notices
// nothing — same output, same prompts, same exit codes, same help.
//
// What is not passed through is a handful of commands prefixed with `hm-`, which the shell
// implementation does not have and never will. They exist to see what the Go layer resolved
// without changing what any real command does.
func Run(args []string, stdout, stderr io.Writer) int {
	runner := legacy.Runner{}

	if len(args) > 0 {
		switch args[0] {
		case "hm-go-version":
			return goVersion(stdout)
		case "hm-go-project":
			return project(args[1:], stdout, stderr)
		}
	}

	//
	// Ported commands. Everything else falls through to the shell implementation, and the list
	// grows one command at a time — each with the tests of the one it replaces, and each checked
	// against it by tests/integration/go_passthrough_test.sh.
	//
	// The global flags are consumed here because the Go side owns the output format now. What is
	// left is passed to the command, which is why an unknown option still reaches it and is still
	// a usage error.
	//
	if len(args) > 0 && args[0] == "list" {
		jsonOutput, rest := wantsJSON(args[1:], stdout)

		return list(rest, stdout, stderr, jsonOutput)
	}

	code, err := runner.Run(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)

		// The exit code for something wrong with the tool itself rather than with the project
		return 3
	}

	return code
}

// goVersion says which binary this is and what it was built from. It is the one question that
// cannot be answered by the shell implementation, because the shell implementation is not the
// thing being asked about.
func goVersion(stdout io.Writer) int {
	version := Version
	if version == "" {
		version = "dev"
	}

	revision := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
				revision = setting.Value[:7]
			}
		}
	}

	fmt.Fprintf(stdout, "%s\n", version)
	if revision != "" {
		fmt.Fprintf(stdout, "%s\n", revision)
	}

	return 0
}

// project prints what the Go layer resolved, as JSON.
//
// It is how the resolution is checked against the shell implementation's, command by command, as
// each one is ported: two answers to the same question, compared by a test rather than by
// somebody reading both.
func project(args []string, stdout, stderr io.Writer) int {
	directory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)

		return 3
	}

	if len(args) > 0 {
		directory = args[0]
	}

	resolver := app.Resolver{
		Properties: fsprops.Reader{},
		VCS:        gitvcs.Git{},
		Registry:   registry{},
	}

	resolved, err := resolver.Resolve(directory)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)

		return 4
	}

	document, err := json.MarshalIndent(map[string]any{
		"name":        resolved.Name,
		"root":        resolved.Root,
		"domain":      resolved.Domain,
		"magento_dir": resolved.MagentoDir,
		"topology":    string(resolved.Topology),
		"worktree":    resolved.Worktree,
	}, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)

		return 3
	}

	fmt.Fprintf(stdout, "%s\n", document)

	return 0
}

// registry is a placeholder while the worktree registry is still owned by the shell
// implementation. It reports no branch environment, which is the truth for every project until
// that part is ported: what it must not do is guess.
type registry struct{}

func (registry) Worktree(string, string) (*core.Worktree, error) { return nil, nil }
