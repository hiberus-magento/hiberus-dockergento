package cli

import "io"

// dockerCompose runs a Compose subcommand this tool does not implement itself, against the
// resolved project's own files and environment, and hands back that subcommand's own exit code.
//
// Transparent, the same as exec: no envelope wraps a successful answer, because Compose owns
// stdout and stderr for the duration of the call.
func dockerCompose(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if _, code := projectOr(stderr, jsonOutput, "docker-compose"); code != 0 {
		return code
	}

	status, err := newEngine(stdout, stderr, jsonOutput).Compose(here(), args)
	if err != nil {
		return report(stderr, jsonOutput, "docker-compose", err)
	}

	return status
}
