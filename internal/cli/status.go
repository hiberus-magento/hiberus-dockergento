package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// status is how the environment is doing, in one call: the containers, whether the URLs answer,
// xdebug, the mode, whether the data is anonymised and the last error each running service
// logged. Text is a compact table; --json the whole answer.
func status(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 {
		return failure(stderr, jsonOutput, "status", exitUsage, "invalid_argument",
			"Unknown option: "+args[0], binaryName()+" status")
	}

	answer, err := newEngine(stdout, stderr, jsonOutput).Status(here())
	if err != nil {
		return report(stderr, jsonOutput, "status", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "status", answer)
	}

	statusAsText(answer, stdout)

	return exitOK
}

func statusAsText(answer core.Status, stdout io.Writer) {
	data := "data not anonymised"
	if answer.Anonymised == "yes" {
		data = "anonymised " + answer.AnonymisedAt
	}

	fmt.Fprintf(stdout, "%s  %s  mode %s  xdebug %s  %s\n",
		header(answer.Project), answer.State, orUnknown(answer.Mode), orUnknown(answer.Xdebug), data)

	fmt.Fprintf(stdout, "\n  %-15s %-12s %s\n", "SERVICE", "STATE", "LAST ERROR")

	for _, service := range answer.Services {
		state := service.State
		if service.Running {
			state = good(state)
		} else {
			state = warning(state)
		}

		fmt.Fprintf(stdout, "  %-15s %s%s %s\n", service.Name, state, statusPad(service.State, 12),
			orDash(service.LastError))
	}

	if len(answer.URLs) == 0 {
		return
	}

	fmt.Fprintf(stdout, "\n  %-15s %-12s %s\n", "URL", "ANSWERS", "ADDRESS")

	for _, url := range answer.URLs {
		answered, address := "no", url.URL
		if url.Answers {
			answered = strconv.Itoa(url.Code)
		} else if url.Error != "" {
			address += " (" + url.Error + ")"
		}

		fmt.Fprintf(stdout, "  %-15s %-12s %s\n", url.Name, answered, address)
	}
}

// statusPad is the space that brings a painted value to width: the colour codes count for %-12s and
// show as nothing, so the padding is worked out from the plain text.
func statusPad(plain string, width int) string {
	if len(plain) >= width {
		return " "
	}

	return fmt.Sprintf("%*s", width-len(plain), "")
}
