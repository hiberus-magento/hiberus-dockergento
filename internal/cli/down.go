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

	for at := 0; at < len(args); at++ {
		argument := args[at]

		switch argument {
		case "-v", "--volumes":
			options.Volumes = true
		case "--remove-orphans":
			options.RemoveOrphans = true
		case "--rmi":
			if at+1 < len(args) {
				at++
				options.Images = args[at]
			}
		case "-t", "--timeout":
			if at+1 < len(args) {
				at++

				seconds, err := strconv.Atoi(args[at])
				if err != nil {
					return failure(stderr, jsonOutput, "down", exitUsage, "invalid_argument",
						"The timeout is a number of seconds: "+args[at],
						binaryName()+" down -t 30")
				}

				options.Timeout = &seconds
			}
		default:
			return failure(stderr, jsonOutput, "down", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" down [-v]")
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "down"); code != 0 {
		return code
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	what, err := engine(stdout, stderr, jsonOutput).Down(here(), options, interactive)
	if err != nil {
		return report(stderr, jsonOutput, "down", err)
	}

	// Nothing happened, because somebody said so
	if what == "" {
		fmt.Fprint(stdout, good("Nothing was destroyed.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "down", map[string]any{
			"destroyed": true, "volumes": options.Volumes, "snapshot": what == "saved",
		})
	}

	return exitOK
}
