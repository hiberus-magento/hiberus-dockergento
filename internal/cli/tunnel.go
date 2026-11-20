package cli

import (
	"fmt"
	"io"
	"strconv"
)

//
// A service of this project reachable from this machine, for as long as the command is left
// running.
//
// The relay is the use case's (dockergento/app/tunnel.go). What is here is the terminal: the
// arguments, what is printed while it is open, and closing it on Ctrl-C.
//

func tunnel(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if _, code := projectOr(stderr, jsonOutput, "tunnel"); code != 0 {
		return code
	}

	closeOnly := false
	positional := []string{}

	for _, argument := range args {
		if argument == "--close" {
			closeOnly = true

			continue
		}

		positional = append(positional, argument)
	}

	service, port := "", 0

	if len(positional) > 0 {
		service = positional[0]
	}

	if len(positional) > 1 {
		number, err := strconv.Atoi(positional[1])
		if err != nil || number <= 0 {
			return failure(stderr, jsonOutput, "tunnel", exitUsage, "invalid_argument",
				fmt.Sprintf("'%s' is not a port", positional[1]),
				binaryName()+" tunnel "+service+" <port>")
		}

		port = number
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	if closeOnly {
		closed, err := engine.CloseTunnels(here(), service)
		if err != nil {
			return report(stderr, jsonOutput, "tunnel", err)
		}

		if jsonOutput {
			return document(stdout, stderr, "tunnel", map[string]bool{"closed": closed})
		}

		if closed {
			fmt.Fprint(stdout, good("Tunnels closed.\n"))
		} else {
			fmt.Fprint(stdout, good("No tunnels were open.\n"))
		}

		return exitOK
	}

	// Taken from before the relay is created, so a Ctrl-C while it is being opened does not leave
	// it publishing a port (interrupt.go). Without a service there is nothing to open — the
	// refusal comes back at once — and closing "every tunnel" on an interruption would be wrong
	var interruptions *interruption
	if service != "" {
		interruptions = closeOnInterrupt(func() { engine.CloseTunnels(here(), service) }) //nolint:errcheck
		interruptions.opening()
	}

	opened, err := engine.Tunnel(here(), service, port)
	interruptions.opened()

	if err != nil {
		return report(stderr, jsonOutput, "tunnel", err)
	}

	interruptions.holding()

	if jsonOutput {
		document(stdout, stderr, "tunnel", opened)
	} else {
		fmt.Fprint(stdout, "\n"+good(opened.Service+" is reachable at ")+
			paint("\033[0;33m", fmt.Sprintf("%s:%d", opened.Host, opened.Port))+"\n\n")
		fmt.Fprint(stdout, "  Leave this running while you use it. Ctrl-C closes it.\n\n")
	}

	engine.HoldTunnel(opened.Relay)      //nolint:errcheck
	engine.CloseTunnels(here(), service) //nolint:errcheck

	// What closing says is not the document: under --json it goes with the other notes
	closing := stdout
	if jsonOutput {
		closing = stderr
	}

	fmt.Fprint(closing, "\n"+good("Tunnel closed.\n"))

	return exitOK
}
