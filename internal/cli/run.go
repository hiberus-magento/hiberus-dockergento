// Package cli is a driving adapter: it turns a command line into a use case and a use case's
// answer into output. It is the only place in the Go tree that knows a terminal exists.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
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
	//
	// Ported commands. Everything else falls through to the shell implementation, and the list
	// grows one command at a time — each with the tests of the one it replaces, and each checked
	// against it by tests/integration/go_passthrough_test.sh.
	//
	// The tool's own flags are taken out first, wherever they appear, because the Go side owns
	// the output format now. What is left is the command and its own arguments, which is why an
	// option nobody declared still reaches the command and is still a usage error.
	//
	format, rest := globals(args)

	if len(rest) > 0 {
		jsonOutput := wanted(format, stdout)

		switch rest[0] {
		// Two commands the shell implementation does not have and never will. They exist to see
		// what the Go layer resolved, without changing what any real command does.
		case "hm-go-version":
			return goVersion(stdout)
		case "hm-go-project":
			return project(rest[1:], stdout, stderr)
		case "hm-go-registry":
			return registryState(stdout, stderr)
		case "list":
			return list(rest[1:], stdout, stderr, jsonOutput)
		case "describe":
			return describe(rest[1:], stdout, stderr, jsonOutput)
		case "web":
			return web(rest[1:], stdout, stderr, jsonOutput)
		case "doctor":
			return doctor(rest[1:], stdout, stderr, jsonOutput)
		case "stop":
			return stop(rest[1:], stdout, stderr, jsonOutput)
		case "logs":
			return logs(rest[1:], stdout, stderr, jsonOutput)
		case "magento":
			return magento(rest[1:], stdout, stderr, jsonOutput)
		case "composer":
			// The invocation that rewrites the host's dependency tree stays with the shell
			// implementation, and only that one
			if !mirrorsVendor(rest[1:]) {
				return composer(rest[1:], stdout, stderr, jsonOutput)
			}
		case "exec":
			// Everything after the command belongs to the command, so the global flags are not
			// consumed here: `hm exec grep --json` is asking grep for --json
			return execute(rest[1:], stdout, stderr, jsonOutput)
		case "start":
			return start(rest[1:], stdout, stderr, jsonOutput)
		case "restart":
			return restart(rest[1:], stdout, stderr, jsonOutput)
		}
	}

	code, err := engine(stdout, stderr, false).Shell(args)
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

	resolved, err := engine(stdout, stderr, false).Resolve(directory)
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
