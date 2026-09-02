package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// The database client.
//
// What is here is what the shell implementation does before it reaches the client: refuse when
// the database is not running, and decide which of the three things is being asked for — a
// statement, a dump on the input, or a session.
//
// What is not here is the import. `-i` also cleans DEFINER clauses, optionally anonymises, and
// then configures Magento for local development — which prompts for a domain and writes into the
// project. That sequence stays whole with the shell implementation until the things it depends on
// are ported.
func mysql(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	statement := ""

	for at := 0; at < len(args); at++ {
		switch argument := args[at]; argument {
		case "-q":
			if at+1 >= len(args) {
				return usageOfMysql(stderr, jsonOutput)
			}

			at++
			statement = args[at]
		case "-d", "-a":
			// They only mean anything with an import, and an import is not handled here. On
			// their own the shell implementation opens a session, so this does too
		default:
			if strings.HasPrefix(argument, "-") {
				return usageOfMysql(stderr, jsonOutput)
			}
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "mysql"); code != 0 {
		return code
	}

	engine := engine(stdout, stderr, jsonOutput)

	if statement != "" {
		status, err := engine.Query(here(), statement, stdout)
		if err != nil {
			return report(stderr, jsonOutput, "mysql", err)
		}

		return status
	}

	//
	// No statement: a dump on the input, or a session.
	//
	// Only a stdin that is not a terminal counts as a dump. Gating on that alone is what made
	// `-q` unreachable for anything without a terminal — a CI job or an agent — which is why the
	// shell implementation checks that no options were given as well.
	//
	if !isTerminal(os.Stdin) {
		fmt.Fprint(stdout, good("Importing database from stdin ...\n"))

		status, err := engine.Feed(here())
		if err != nil {
			return report(stderr, jsonOutput, "mysql", err)
		}

		return status
	}

	status, err := engine.Console(here())
	if err != nil {
		return report(stderr, jsonOutput, "mysql", err)
	}

	return status
}

func usageOfMysql(stderr io.Writer, jsonOutput bool) int {
	return failure(stderr, jsonOutput, "mysql", exitUsage, "invalid_argument",
		"The command is not correct", binaryName()+" mysql -q \"SELECT 1\"")
}

// importsADump reports whether this invocation is the one that also cleans, anonymises and
// configures Magento afterwards, which is still the shell implementation's.
func importsADump(args []string) bool {
	for _, argument := range args {
		if argument == "-i" {
			return true
		}
	}

	return false
}
