package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
)

// The database client: a statement, a dump, or a session.
//
// An import is not only an import — it strips the DEFINER clauses, clears the record of the data
// having been anonymised, optionally anonymises, and then points the store at this machine. That
// whole sequence is here, because leaving half of it elsewhere is what makes a command mean two
// different things depending on which implementation ran it.
func mysql(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	statement := ""
	options := app.ImportOptions{}

	for at := 0; at < len(args); at++ {
		switch argument := args[at]; argument {
		case "-q":
			if at+1 >= len(args) {
				return usageOfMysql(stderr, jsonOutput)
			}

			at++
			statement = args[at]
		case "-i":
			if at+1 >= len(args) {
				return usageOfMysql(stderr, jsonOutput)
			}

			at++
			options.File = expandHome(args[at])
		case "-d":
			options.CleanDefiners = true
		case "-a":
			options.Anonymise = true
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

	if options.File != "" {
		if _, err := os.Stat(options.File); err != nil {
			// Not an error the way a broken command is: the shell implementation says so and
			// stops, which is what somebody who mistyped a path needs
			fmt.Fprint(stderr, warning(fmt.Sprintf("No such file: %s\n", options.File)))

			return exitOK
		}

		return report(stderr, jsonOutput, "mysql", engine.Import(here(), options))
	}

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

// expandHome takes the tilde the shell did not, because the path arrives as a value and not as a
// word the shell expanded.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return home + strings.TrimPrefix(path, "~")
}
