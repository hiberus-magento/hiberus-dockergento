package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func doctor(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	only := ""

	for _, argument := range args {
		if value, found := strings.CutPrefix(argument, "--only="); found {
			only = value

			continue
		}

		return failure(stderr, jsonOutput, "doctor", exitUsage, "invalid_argument",
			fmt.Sprintf("Unknown option: %s", argument), "hm doctor --help")
	}

	// Outside a project the diagnosis still runs: it answers about the machine, which is the
	// question somebody has when nothing works anywhere
	diagnosis, err := engine(stdout, stderr, jsonOutput).Diagnose(here(), only)
	if err != nil {
		return failure(stderr, jsonOutput, "doctor", exitError, "diagnosis_failed", err.Error(), "")
	}

	if jsonOutput {
		document, err := json.MarshalIndent(envelope{
			SchemaVersion: 1,
			Command:       "doctor",
			OK:            true,
			Data:          diagnosis,
		}, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)

			return exitError
		}

		fmt.Fprintf(stdout, "%s\n", document)
	} else {
		diagnosisAsText(diagnosis, stdout)
	}

	// A diagnosis that found something broken fails, so that a script or a CI job does not have
	// to read the report to know
	if diagnosis.Summary.Errors > 0 {
		return exitError
	}

	return exitOK
}

func diagnosisAsText(diagnosis core.Diagnosis, stdout io.Writer) {
	fmt.Fprintf(stdout, "\n")

	for _, finding := range diagnosis.Checks {
		switch finding.Severity {
		case core.SeverityOK:
			fmt.Fprint(stdout, good("  OK   "))
		case core.SeverityWarning:
			fmt.Fprint(stdout, warning("  WARN "))
		case core.SeverityError:
			fmt.Fprint(stdout, bad("  FAIL "))
		}

		fmt.Fprintf(stdout, "%-20s %s\n", finding.ID, finding.Message)

		if finding.Action != "" && finding.Severity != core.SeverityOK {
			fmt.Fprintf(stdout, "       %-18s ", "")
			fmt.Fprint(stdout, warning(finding.Action+"\n"))
		}
	}

	fmt.Fprintf(stdout, "\n")

	switch {
	case diagnosis.Summary.Errors > 0:
		fmt.Fprint(stdout, bad(fmt.Sprintf("  %d error(s), %d warning(s)\n\n",
			diagnosis.Summary.Errors, diagnosis.Summary.Warnings)))
	case diagnosis.Summary.Warnings > 0:
		fmt.Fprint(stdout, warning(fmt.Sprintf("  No blocking problems, %d warning(s)\n\n",
			diagnosis.Summary.Warnings)))
	default:
		fmt.Fprint(stdout, good("  Everything looks good\n\n"))
	}
}
