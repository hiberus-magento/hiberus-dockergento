package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/composecfg"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/dockerd"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/machine"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/magentofiles"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/osfs"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/toolinfo"
	"github.com/hiberus-magento/hiberus-dockergento/internal/app"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
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
	project, err := resolveProject()
	inProject := err == nil && project.Name != "" &&
		(osfs.FS{}).Exists(filepath.Join(project.Root, "config", "docker", "properties.json"))

	physician := app.Doctor{
		Daemon:  dockerd.Daemon{Timeout: 10 * time.Second},
		Engine:  dockerd.Engine{Timeout: 10 * time.Second},
		Compose: composecfg.Loader{Environment: environmentFor(project)},
		Magento: magentofiles.Reader{},
		Tooling: toolinfo.Reader{Root: hmRoot()},
		State:   toolinfo.State{Dir: os.Getenv("HM_STATE_DIR")},
		Machine: machine.Host{},
		FS:      osfs.FS{},

		Project:      project,
		InProject:    inProject,
		ComposeFiles: composeFilesFor(project),
		Template:     filepath.Join(hmRoot(), "docker-compose", "docker-compose.template.yml"),
		Platform:     machineName(),
		Binary:       binaryName(),
		Profile:      os.Getenv("HM_PROFILE"),
		Agent:        os.Getenv("HM_AGENT"),
	}

	diagnosis := physician.Diagnose(only)

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

// binaryName is what the tool was invoked as, because every action printed is a command the
// reader is meant to be able to paste.
func binaryName() string {
	if name := os.Getenv("COMMAND_BIN_NAME"); name != "" {
		return name
	}

	return "hm"
}
