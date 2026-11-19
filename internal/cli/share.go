package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

//
// A public address for this project, for as long as the command is left running.
//
// The tunnel is the use case's (dockergento/app/share.go). What is here is the terminal: the one
// question in this tool that protects something outside the machine, what is printed while it is
// open, and closing it on Ctrl-C.
//

func share(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if _, code := projectOr(stderr, jsonOutput, "share"); code != 0 {
		return code
	}

	stopOnly := false

	for _, argument := range args {
		if argument != "--stop" {
			return failure(stderr, jsonOutput, "share", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" share [--stop]")
		}

		stopOnly = true
	}

	if stopOnly {
		return unshare(stdout, stderr, jsonOutput)
	}

	service := os.Getenv("HM_SHARE_SERVICE")
	if service == "" {
		service = "varnish"
	}

	port, err := strconv.Atoi(envOr("HM_SHARE_PORT", "6081"))
	if err != nil {
		return failure(stderr, jsonOutput, "share", exitUsage, "invalid_argument",
			"HM_SHARE_PORT is not a port: "+os.Getenv("HM_SHARE_PORT"), "")
	}

	var confirm func(string) bool
	if os.Getenv("HM_NON_INTERACTIVE") == "" {
		confirm = func(project string) bool { return confirmSharing(project, stdout) }
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	shared, err := engine.Share(here(), service, port, confirm)
	if err != nil {
		return report(stderr, jsonOutput, "share", err)
	}

	if shared.Declined {
		return exitOK
	}

	// Closed on Ctrl-C or a terminal going away, which is what ends the wait below
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(interrupted)

	go func() {
		if _, open := <-interrupted; open {
			engine.Unshare(here()) //nolint:errcheck
		}
	}()

	if jsonOutput {
		document(stdout, stderr, "share", map[string]string{"url": shared.URL, "project": shared.Project})
	} else {
		fmt.Fprint(stdout, "\n"+good(fmt.Sprintf("'%s' is reachable at ", shared.Project))+link(shared.URL)+"\n\n")
		fmt.Fprint(stdout, "  The address changes every time, and it is gone when you stop.\n")
		fmt.Fprint(stdout, "  Magento builds its links from base_url, so they still point at your local\n")
		fmt.Fprint(stdout, "  domain. That matters for a demo, not for receiving webhooks.\n\n")
		fmt.Fprint(stdout, "  Leave this running while you use it. Ctrl-C stops sharing.\n\n")
	}

	engine.HoldShare(shared.Relay) //nolint:errcheck
	engine.Unshare(here())         //nolint:errcheck

	// What closing says is not the document: under --json it goes with the other notes
	closing := stdout
	if jsonOutput {
		closing = stderr
	}

	fmt.Fprint(closing, "\n"+good("Sharing stopped.\n"))

	return exitOK
}

func unshare(stdout, stderr io.Writer, jsonOutput bool) int {
	closed, err := newEngine(stdout, stderr, jsonOutput).Unshare(here())
	if err != nil {
		return report(stderr, jsonOutput, "share", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "share", map[string]bool{"closed": closed})
	}

	if closed {
		fmt.Fprint(stdout, good("The tunnel is closed.\n"))
	} else {
		fmt.Fprint(stdout, good("Nothing was being shared.\n"))
	}

	return exitOK
}

// confirmSharing names what is being exposed rather than asking "are you sure?": while the tunnel
// is open, anyone with the address reaches the environment, admin panel and data included.
func confirmSharing(project string, stdout io.Writer) bool {
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, warning(fmt.Sprintf("This puts '%s' on the public internet.\n", project)))
	fmt.Fprint(stdout, warning("Anyone with the address reaches it — the admin panel and the data in it.\n\n"))

	answer, err := ask("Share it? [y/N]:", "")

	return err == nil && strings.EqualFold(strings.TrimSpace(answer), "y")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
