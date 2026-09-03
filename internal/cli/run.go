// Package cli is a driving adapter: it turns a command line into a use case and a use case's
// answer into output. It is the only place in the Go tree that knows a terminal exists.
package cli

import (
	"fmt"
	"io"
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
		//
		// Scaffolding, and marked as such: the underscore is what this tool already uses in
		// `data/command_descriptions.json` for keys that are not commands, and these are not
		// commands. They are not in the help, not in the command list, and they exist for one
		// reason — while the discipline is byte-for-byte parity with the shell implementation,
		// a ported command cannot report anything the shell one does not, so what only the Go
		// layer knows has nowhere else to be looked at.
		//
		// Each has a condition for disappearing, written down in MIGRATION.md:
		//
		//   _binary    goes into `hm version` when `version` is ported
		//   _registry  goes into `hm worktree list` when `worktree` is ported
		//
		case "_binary":
			return goVersion(stdout)
		case "_registry":
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
		case "clean":
			return clean(rest[1:], stdout, stderr, jsonOutput)
		case "worktree":
			// All three are ported, which is what will let the registry change underneath them:
			// while one half wrote JSON and the other read SQLite there would be a branch
			// environment `add` created that nothing else could see
			if len(rest) > 1 && worktreeSubcommands[rest[1]] {
				return worktree(rest[1:], stdout, stderr, jsonOutput)
			}
		case "db":
			// Both families of it. What is left over goes to the shell implementation, which is
			// where the usage text and an unknown subcommand are still answered
			if len(rest) > 1 && templateSubcommands[rest[1]] {
				return db(rest[1:], stdout, stderr, jsonOutput)
			}
		case "bash":
			return shell(rest[1:], stdout, stderr, jsonOutput)
		case "masquerade":
			return masquerade(rest[1:], stdout, stderr, jsonOutput)
		case "mysql":
			return mysql(rest[1:], stdout, stderr, jsonOutput)
		case "exec":
			// Everything after the command belongs to the command, so the global flags are not
			// consumed here: `hm exec grep --json` is asking grep for --json
			return execute(rest[1:], stdout, stderr, jsonOutput)
		case "down":
			return down(rest[1:], stdout, stderr, jsonOutput)
		case "start":
			return start(rest[1:], stdout, stderr, jsonOutput)
		case "restart":
			return restart(rest[1:], stdout, stderr, jsonOutput)
		}
	}

	code, err := engine(stdout, stderr, false).Shell(here(), args)
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
