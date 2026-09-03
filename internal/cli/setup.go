package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Creating a project's environment.
//
// The whole instruction is read before anything is created, which is the fix for the failure that
// mattered: a dump path that does not exist used to be a warning, after which the command carried
// on and asked the question interactively — so an automated bootstrap with a wrong path hung
// instead of failing.
//

func setup(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options, err := core.ParseSetup(args, binaryName())
	if err != nil {
		return report(stderr, jsonOutput, "setup", err)
	}

	engine := engine(stdout, stderr, jsonOutput)

	if err := engine.Setup(here(), options, os.Getenv("HM_NON_INTERACTIVE") == ""); err != nil {
		return report(stderr, jsonOutput, "setup", err)
	}

	project, err := engine.Resolve(here())
	if err != nil {
		return report(stderr, jsonOutput, "setup", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "setup", map[string]any{
			"project": project.Name, "domain": project.Domain,
			"magento_dir": project.MagentoDir,
		})
	}

	fmt.Fprint(stdout, good("\nSetup completed!!!\n\n"))
	fmt.Fprint(stdout, good("Open "))
	fmt.Fprint(stdout, link("https://"+project.Domain+"/\n\n"))

	return exitOK
}
