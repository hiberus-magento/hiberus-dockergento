package cli

import (
	"fmt"
	"io"
	"os/exec"
)

// lookOpener finds what opens an address in the browser: open on macOS, xdg-open elsewhere, the
// order the shell asked in. A variable so that a test never opens a browser.
var lookOpener = func() (string, bool) {
	for _, name := range []string{"open", "xdg-open"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}

	return "", false
}

// launchTargets are the destinations launch knows, by option, with the two names the mail catcher
// has had folded into one.
var launchTargets = map[string]string{
	"--base": "base", "--admin": "admin", "--mail": "mail", "--mailhog": "mail", "--mailpit": "mail",
	"--rabbitmq": "rabbitmq", "--search": "search",
}

// launch opens one of the project's addresses in the browser: the store, unless another
// destination is asked for. Under --json it says where instead, and opens nothing.
func launch(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	target, chosen := "base", ""

	for _, argument := range args {
		named, known := launchTargets[argument]
		if !known {
			return failure(stderr, jsonOutput, "launch", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), binaryName()+" launch --help")
		}

		if chosen != "" {
			return failure(stderr, jsonOutput, "launch", exitUsage, "conflicting_options",
				fmt.Sprintf("Pick one destination: %s and %s cannot both be opened", chosen, argument),
				binaryName()+" launch --admin")
		}

		chosen, target = argument, named
	}

	if _, code := projectOr(stderr, jsonOutput, "launch"); code != 0 {
		return code
	}

	description, err := newEngine(stdout, stderr, jsonOutput).Describe(here(), false)
	if err != nil {
		return report(stderr, jsonOutput, "launch", err)
	}

	urls := description.Project.URLs
	url := map[string]string{
		"base": urls.Base, "admin": urls.Admin, "mail": urls.Mail, "rabbitmq": urls.RabbitMQ,
		"search": urls.Search,
	}[target]

	if url == "" {
		if target == "base" || target == "admin" {
			return failure(stderr, jsonOutput, "launch", exitProject, "no_domain",
				"This project has no domain configured, so it has no address to open",
				binaryName()+" setup --domain=project.local")
		}

		return failure(stderr, jsonOutput, "launch", exitService, "no_address",
			fmt.Sprintf("The %s service publishes no port, so it has no address to open", target),
			binaryName()+" describe")
	}

	if jsonOutput {
		return document(stdout, stderr, "launch", map[string]any{"target": target, "url": url, "opened": false})
	}

	opener, found := lookOpener()
	if !found {
		fmt.Fprint(stdout, warning("No browser opener on this machine. The address is:\n"))
		fmt.Fprintf(stdout, "%s\n", url)

		return exitOK
	}

	if err := openWith(opener, url); err != nil {
		return failure(stderr, jsonOutput, "launch", exitError, "open_failed",
			"Could not open "+url, binaryName()+" launch --json")
	}

	fmt.Fprint(stdout, "Opened ")
	fmt.Fprint(stdout, link(url+"\n"))

	return exitOK
}
