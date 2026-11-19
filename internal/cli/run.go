// Package cli is a driving adapter: it turns a command line into a use case and a use case's
// answer into output. It is the only place in the Go tree that knows a terminal exists.
package cli

import (
	"fmt"
	"io"
	"os"
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

	//
	// `hm <command> --help` is the command's help, for every command and before the command runs:
	// bin/run answers it from data/command_descriptions.json, and a handler handed it as an
	// argument would answer "unknown subcommand" or "not a project" instead.
	//
	if len(rest) > 0 && !(len(rest) > 1 && (rest[1] == "--help" || rest[1] == "-h")) {
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
		//   _registry  goes when nothing in shell needs to read the registry: it is also how
		//              `bin/run` reads it, now that the registry is a database
		//
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
		case "permissions":
			return permissions(rest[1:], stdout, stderr, jsonOutput)
		case "ai-context":
			return aiContext(rest[1:], stdout, stderr, jsonOutput)
		case "mcp":
			// The protocol is on standard input, which is the one stream Run is not handed
			return mcp(rest[1:], os.Stdin, stdout, stderr, jsonOutput)
		case "launch":
			return launch(rest[1:], stdout, stderr, jsonOutput)
		case "transfer-media":
			return transferMedia(rest[1:], stdout, stderr, jsonOutput)
		case "transfer-db":
			return transferDB(rest[1:], stdout, stderr, jsonOutput)
		case "grunt":
			return grunt(rest[1:], stdout, stderr, jsonOutput)
		case "update":
			return update(rest[1:], stdout, stderr, jsonOutput)
		case "config-env":
			return configEnv(rest[1:], stdout, stderr, jsonOutput)
		case "cloud":
			return cloud(rest[1:], stdout, stderr, jsonOutput)
		case "cloud-login":
			return cloudLogin(rest[1:], stdout, stderr, jsonOutput)
		case "composer":
			// All of it, the macOS vendor dance included: composer.sh has nothing left to answer
			return composer(rest[1:], stdout, stderr, jsonOutput)
		case "clean":
			return clean(rest[1:], stdout, stderr, jsonOutput)
		case "worktree":
			// All of it, including no subcommand and one it does not know: the shell twin is
			// retired, and while it existed a fallthrough reached worktree.sh, which exec'd this
			// binary again with the same arguments — a loop for as long as Docker was up
			return worktree(rest[1:], stdout, stderr, jsonOutput)
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
		case "set-host":
			return setHost(rest[1:], stdout, stderr, jsonOutput)
		case "copy-to-container":
			return copyInto(rest[1:], stdout, stderr, jsonOutput)
		case "copy-from-container":
			return copyFrom(rest[1:], stdout, stderr, jsonOutput)
		case "version":
			return version(rest[1:], stdout, stderr, jsonOutput)
		case "purge":
			return purge(stdout, stderr, jsonOutput)
		case "npm":
			return npm(rest[1:], stdout, stderr, jsonOutput)
		case "n98-magerun":
			return magerun(rest[1:], stdout, stderr, jsonOutput)
		case "test-unit":
			return tests("unit", rest[1:], stdout, stderr, jsonOutput)
		case "test-integration":
			return tests("integration", rest[1:], stdout, stderr, jsonOutput)
		case "mysqldump":
			return dump(rest[1:], stdout, stderr, jsonOutput)
		case "varnish-on":
			return varnish(true, stdout, stderr, jsonOutput)
		case "varnish-off":
			return varnish(false, stdout, stderr, jsonOutput)
		case "debug-off":
			return debugOff(stdout, stderr, jsonOutput)
		case "debug-on":
			return debugOn(stdout, stderr, jsonOutput)
		case "setup":
			return setup(rest[1:], stdout, stderr, jsonOutput)
		case "create-project":
			return createProject(rest[1:], stdout, stderr, jsonOutput)
		case "install":
			return install(rest[1:], stdout, stderr, jsonOutput)
		case "proxy":
			return proxy(rest[1:], stdout, stderr, jsonOutput)
		case "share":
			return share(rest[1:], stdout, stderr, jsonOutput)
		case "tunnel":
			return tunnel(rest[1:], stdout, stderr, jsonOutput)
		case "compatibility":
			return compatibility(stdout, stderr, jsonOutput)
		case "switch":
			return switchVersion(rest[1:], stdout, stderr, jsonOutput)
		case "verify":
			return verify(rest[1:], stdout, stderr, jsonOutput)
		case "start":
			return start(rest[1:], stdout, stderr, jsonOutput)
		case "restart":
			return restart(rest[1:], stdout, stderr, jsonOutput)
		case "post-start":
			return postStart(stdout, stderr, jsonOutput)
		case "ssl":
			return ssl(rest[1:], stdout, stderr, jsonOutput)
		case "rebuild":
			return rebuild(rest[1:], stdout, stderr, jsonOutput)
		case "docker-stop-all":
			return dockerStopAll(rest[1:], stdout, stderr, jsonOutput)
		case "docker-compose":
			return dockerCompose(rest[1:], stdout, stderr, jsonOutput)
		case "dbeaver":
			return dbClient("dbeaver", rest[1:], stdout, stderr, jsonOutput)
		case "sequelace":
			return dbClient("sequelace", rest[1:], stdout, stderr, jsonOutput)
		case "tableplus":
			return dbClient("tableplus", rest[1:], stdout, stderr, jsonOutput)
		case "tui":
			// Arguments are ignored, as tui.sh ignored them: the only question is whether there
			// is a terminal to draw on
			return tui(stdout, stderr, jsonOutput)
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
