package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// engine builds the tool for this invocation.
//
// Everything the command line adds is here and nowhere else: the name it was invoked as, which
// goes into every hint, and where the steps that take a while announce themselves. The engine
// itself has no opinion about terminals — which is what lets the same calls answer an HTTP request
// later without any of this coming along.
func engine(stdout, stderr io.Writer, jsonOutput bool) *dockergento.Engine {
	return dockergento.New(dockergento.Options{
		Binary:   binaryName(),
		StateDir: os.Getenv("HM_STATE_DIR"),

		// --force is a decision about one invocation, which is why it is read from the
		// environment the flag set and never from a file
		Forced: os.Getenv("HM_FORCE") == "1",

		// In JSON mode stdout carries the document, so anything decorative goes to stderr:
		// otherwise a program reading the output finds a sentence in the middle of it
		Announce: func(message string) {
			where := stdout
			if jsonOutput {
				where = stderr
			}

			fmt.Fprint(where, good(message))
		},

		// The spinner and the question live here for the same reason: the engine has no terminal
		// and should not learn about one
		Progress: func(label string) func(bool, string) {
			return begin(label, stdout, stderr, jsonOutput)
		},
		Ask: ask,
	})
}

// projectOr resolves the project this directory belongs to, or reports why it could not.
func projectOr(stderr io.Writer, jsonOutput bool, command string) (core.Project, int) {
	directory, err := os.Getwd()
	if err != nil {
		return core.Project{}, failure(stderr, jsonOutput, command, exitError, "no_working_directory",
			err.Error(), "")
	}

	project, err := engine(nil, nil, jsonOutput).Resolve(directory)
	if err != nil || project.Name == "" {
		return core.Project{}, failure(stderr, jsonOutput, command, exitProject, "project_not_configured",
			"This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid",
			binaryName()+" setup")
	}

	return project, 0
}

// here is the directory the command was run in, which is what every question is asked about.
func here() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}

	return directory
}

// binaryName is what the tool was invoked as, because every action printed is a command the
// reader is meant to be able to paste.
func binaryName() string {
	if name := os.Getenv("COMMAND_BIN_NAME"); name != "" {
		return name
	}

	return "hm"
}

// registryState prints what the registry holds.
//
// One of the `hm-`prefixed diagnostics: it exists so the registry can be looked at while nothing
// else shows it, and it changes nothing that any real command does.
func registryState(stdout, stderr io.Writer) int {
	held, err := engine(stdout, stderr, false).RegistryState()
	if err != nil {
		fmt.Fprintln(stderr, err)

		return exitError
	}

	document, err := json.MarshalIndent(contract.Success("registry", held), "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)

		return exitError
	}

	fmt.Fprintf(stdout, "%s\n", document)

	return exitOK
}
