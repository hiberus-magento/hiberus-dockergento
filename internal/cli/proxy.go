package cli

import (
	"fmt"
	"io"
)

//
// The global proxy: one per machine, so several projects can be up at once.
//
// It is the only command here that acts on something no project owns, which is why it takes no
// project: a machine has one proxy, and standing in a directory does not change which.
//

var proxySubcommands = map[string]bool{"up": true, "down": true, "status": true}

func proxy(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	subcommand := "status"
	if len(args) > 0 {
		subcommand = args[0]
	}

	switch subcommand {
	case "up":
		return proxyUp(stdout, stderr, jsonOutput)
	case "down":
		return proxyDown(stdout, stderr, jsonOutput)
	case "status":
		return proxyStatus(stdout, stderr, jsonOutput)
	}

	return exitOK
}

func proxyUp(stdout, stderr io.Writer, jsonOutput bool) int {
	started, err := engine(stdout, stderr, jsonOutput).ProxyUp()
	if err != nil {
		return report(stderr, jsonOutput, "proxy", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "proxy", map[string]any{
			"running": true, "started": started,
		})
	}

	if !started {
		fmt.Fprint(stdout, good("The proxy is already running.\n"))

		return exitOK
	}

	fmt.Fprint(stdout, good("Ready. It listens on 80 and 443, and routes by domain.\n"))

	return exitOK
}

func proxyDown(stdout, stderr io.Writer, jsonOutput bool) int {
	stopped, err := engine(stdout, stderr, jsonOutput).ProxyDown()
	if err != nil {
		return report(stderr, jsonOutput, "proxy", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "proxy", map[string]any{
			"running": false, "stopped": stopped,
		})
	}

	if !stopped {
		fmt.Fprint(stdout, good("The proxy is not running.\n"))

		return exitOK
	}

	fmt.Fprint(stdout, good("Stopped.\n"))

	return exitOK
}

func proxyStatus(stdout, stderr io.Writer, jsonOutput bool) int {
	state, err := engine(stdout, stderr, jsonOutput).ProxyStatus()
	if err != nil {
		return report(stderr, jsonOutput, "proxy", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "proxy", map[string]any{
			"running": state.Running, "network": state.Network, "routes": state.Routes,
		})
	}

	fmt.Fprint(stdout, "\n")

	if !state.Running {
		fmt.Fprintf(stdout, "%s\n", header("The proxy is not running\n"))
		fmt.Fprintf(stdout, "  %s proxy up\n\n", binaryName())

		return exitOK
	}

	fmt.Fprintf(stdout, "%s\n", header("The proxy is running\n"))

	if len(state.Routes) == 0 {
		fmt.Fprint(stdout, "  Nothing is routed through it yet.\n\n")

		return exitOK
	}

	for _, route := range state.Routes {
		fmt.Fprintf(stdout, "  %-38s %s\n", "https://"+route.Host+"/", route.Status)
	}

	fmt.Fprint(stdout, "\n")

	return exitOK
}
