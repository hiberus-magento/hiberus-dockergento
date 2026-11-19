package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Creating a new Magento project and its environment.
//
// Like setup, the whole instruction is read before anything is created: an edition that does not
// exist is refused before Magento is downloaded, not after.
//

func createProject(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options, err := core.ParseCreateProject(args, binaryName())
	if err != nil {
		return report(stderr, jsonOutput, "create-project", err)
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	if err := engine.CreateProject(here(), options, os.Getenv("HM_NON_INTERACTIVE") == ""); err != nil {
		return report(stderr, jsonOutput, "create-project", err)
	}

	project, err := engine.Resolve(here())
	if err != nil {
		return report(stderr, jsonOutput, "create-project", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "create-project", map[string]any{
			"project": project.Name, "domain": project.Domain,
			"magento_dir": project.MagentoDir,
		})
	}

	fmt.Fprint(stdout, good("Open "))
	fmt.Fprint(stdout, link("https://"+project.Domain+"/\n"))

	return exitOK
}
