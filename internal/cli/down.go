package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Stop and remove the environment.
//
// Without `-v` this destroys nothing that cannot be rebuilt. With it, the volumes go and the
// database with them: one letter of difference, no warning and no way back — which is why the
// engine asks, and why what it asks is offered as a list rather than as a yes.
//

func down(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options := core.DownOptions{}
	dryRun := false

	for at := 0; at < len(args); at++ {
		argument := args[at]

		switch argument {
		case "--dry-run":
			dryRun = true
		case "-v", "--volumes":
			options.Volumes = true
		case "--remove-orphans":
			options.RemoveOrphans = true
		case "--rmi":
			if at+1 >= len(args) {
				return failure(stderr, jsonOutput, "down", exitUsage, "missing_value",
					"--rmi needs a value", binaryName()+" down --rmi all")
			}

			at++
			options.Images = args[at]
		case "-t", "--timeout":
			if at+1 >= len(args) {
				return failure(stderr, jsonOutput, "down", exitUsage, "missing_value",
					argument+" needs a value", binaryName()+" down -t 30")
			}

			at++

			seconds, err := strconv.Atoi(args[at])
			if err != nil {
				return failure(stderr, jsonOutput, "down", exitUsage, "invalid_argument",
					"The timeout is a number of seconds: "+args[at],
					binaryName()+" down -t 30")
			}

			options.Timeout = &seconds
		default:
			return failure(stderr, jsonOutput, "down", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" down [-v] [--dry-run]")
		}
	}

	project, code := projectOr(stderr, jsonOutput, "down")
	if code != 0 {
		return code
	}

	if dryRun {
		return downPreview(project.Name, options, stdout, stderr, jsonOutput)
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	what, err := newEngine(stdout, stderr, jsonOutput).Down(here(), options, interactive)
	if err != nil {
		return report(stderr, jsonOutput, "down", err)
	}

	// Nothing happened, because somebody said so; under --json that is said in the document, since
	// a sentence on stdout is not one
	if what == "" && !jsonOutput {
		fmt.Fprint(stdout, good("Nothing was destroyed.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "down", map[string]any{
			"destroyed": what != "", "volumes": options.Volumes, "snapshot": what == "saved",
		})
	}

	return exitOK
}

// downPreview says what down would remove, and asks nothing: there is no question to put when
// nothing is going to happen.
func downPreview(project string, options core.DownOptions, stdout, stderr io.Writer, jsonOutput bool) int {
	preview, err := newEngine(stdout, stderr, jsonOutput).DownPreview(here(), options)
	if err != nil {
		return report(stderr, jsonOutput, "down", err)
	}

	removed := removal{containers: preview.Containers, volumes: preview.Volumes}

	if jsonOutput {
		return dryRunDocument(stdout, stderr, "down", project, removed, nil)
	}

	asked := binaryName() + " down"
	if options.Volumes {
		asked += " -v"
	}

	fmt.Fprintf(stdout, "\n%s would remove, in '%s':\n\n", asked, project)

	if removed.empty() {
		fmt.Fprint(stdout, "  nothing: none of its containers or volumes is there\n")
	}

	removed.print(stdout)
	fmt.Fprint(stdout, good("\nNothing was removed.\n"))

	return exitOK
}
