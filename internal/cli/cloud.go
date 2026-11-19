package cli

import (
	"fmt"
	"io"
	"strings"
)

//
// The Magento Cloud CLI lives in the php container, so both commands run it there.
//

// cloud runs the Magento Cloud CLI with its arguments.
//
// Joined into one string through a shell, as the shell implementation did (`bash -c
// "magento-cloud $@"`): the CLI's own argument handling is what people have written their
// invocations against, quoting included.
func cloud(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "cloud")
	if code != 0 {
		return code
	}

	return inside(project, []string{"bash", "-c", strings.TrimSpace("magento-cloud " + strings.Join(args, " "))},
		"", stdout, stderr, jsonOutput, "cloud")
}

// cloudLogin says where an API token is generated and runs the CLI's own login, which asks for it.
func cloudLogin(_ []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "cloud-login")
	if code != 0 {
		return code
	}

	fmt.Fprint(stdout, "Access to following link and generate a new API Token for Magento Cloud CLI:\n")
	fmt.Fprint(stdout, link("https://accounts.magento.cloud/user/api-tokens\n\n"))

	return inside(project, []string{"bash", "-c", "magento-cloud auth:api-token-login"},
		"", stdout, stderr, jsonOutput, "cloud-login")
}
