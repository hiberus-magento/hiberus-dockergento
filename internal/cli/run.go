// Package cli is a driving adapter: it turns a command line into a use case and a use case's
// answer into output. It is the only place in the Go tree that knows a terminal exists.
package cli

import (
	"io"
	"os"
)

// Version is stamped at build time. Empty means a build nobody released.
var Version = ""

// Run dispatches one invocation and returns the exit code.
//
// Every invocation is answered here: the help, the version, every command of the tool, and the
// commands a project adds of its own under config/hm/commands, which are run the way bin/run ran
// them. Nothing falls through to the shell entry point any more.
func Run(args []string, stdout, stderr io.Writer) int {
	//
	// The tool's own flags are taken out first, wherever they appear, because the Go side owns
	// the output format. What is left is the command and its own arguments, which is why an
	// option nobody declared still reaches the command and is still a usage error.
	//
	format, rest := globals(args)

	//
	// The help, then the version, in the order process_hm_options.sh asked: nothing at all is the
	// list of commands, and so is a first argument that asks for help. `hm <command> --help` is the
	// command's help, for every command and before the command runs — a handler handed it as an
	// argument would answer "unknown subcommand" or "not a project" instead.
	//
	if len(rest) == 0 || isHelpFlag(rest[0]) {
		return help("", stdout, stderr)
	}

	if len(rest) > 1 && isHelpFlag(rest[1]) {
		return help(rest[0], stdout, stderr)
	}

	jsonOutput := wanted(format, stdout)

	if rest[0] == "--version" || rest[0] == "-v" {
		return versionFlag(stdout, stderr, jsonOutput)
	}

	switch rest[0] {
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
	case "ai-doctor":
		return aiDoctor(rest[1:], stdout, stderr, jsonOutput)
	case "ai-init":
		return aiInit(rest[1:], stdout, stderr, jsonOutput)
	case "ai-pull":
		return aiPull(rest[1:], stdout, stderr, jsonOutput)
	case "ai-reset":
		return aiReset(rest[1:], stdout, stderr, jsonOutput)
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
		// All of it: both families, the usage text and a subcommand it does not know
		return db(rest[1:], stdout, stderr, jsonOutput)
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
	case "completion":
		return completion(rest[1:], stdout, stderr, jsonOutput)
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

	//
	// Not a command of the tool: one of the project's own, or nothing at all.
	//
	return projectCommand(rest[0], rest[1:], stdout, stderr, jsonOutput)
}

func isHelpFlag(argument string) bool {
	return argument == "--help" || argument == "-h"
}
