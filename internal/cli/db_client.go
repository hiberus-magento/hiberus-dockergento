package cli

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Opening a GUI client on this project's database: `dbeaver`, `sequelace` and `tableplus`.
//
// Connecting one by hand means finding four things the tool already knows and nobody remembers,
// so people keep a saved connection per project and it goes stale the moment a port changes.
// Everything here is read from the description `describe --with-secrets` gives, so a project that
// renamed its database or moved its port gets the right connection without anybody updating a
// profile.
//
// Ported from console/tasks/db_client.sh (hm_db_client_run and the functions it calls): the three
// launchers differ in one word, which is why they share this handler.

// connection is what a client needs to reach the database, in the order the document lists it.
// The port is a string because that is what the shell's `jq --arg` produced.
type connection struct {
	URL      string `json:"url"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
}

// openWith runs the command that opens a client. A variable so that a test can see what would be
// launched without launching anything; production never sets it. Its output is discarded, as the
// shell discarded it.
var openWith = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func dbClient(client string, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	printOnly := false
	forcedPort := ""
	usage := binaryName() + " " + client + " [--print] [--port=<port>]"

	for at := 0; at < len(args); at++ {
		switch argument := args[at]; {
		case argument == "--print":
			printOnly = true
		case strings.HasPrefix(argument, "--port="):
			forcedPort = strings.TrimPrefix(argument, "--port=")
		case argument == "--port" && at+1 < len(args):
			at++
			forcedPort = args[at]
		default:
			return failure(stderr, jsonOutput, client, exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), usage)
		}
	}

	description, err := newEngine(stdout, stderr, jsonOutput).Describe(here(), true)
	if err != nil {
		// As describe answers it: a refusal as the engine gave it, and anything else as the
		// failure it is, never as a project that is not configured
		return report(stderr, jsonOutput, client, err)
	}

	if !serviceRunning(description, "db") {
		return failure(stderr, jsonOutput, client, exitService, "service_not_running",
			"Service 'db' is not running", binaryName()+" start db")
	}

	reach := connectionOf(description, forcedPort)
	if reach.Port == "" {
		// What the global proxy does to it: MySQL carries no hostname, Traefik cannot route it, so
		// the overlay removes the published port altogether
		return failure(stderr, jsonOutput, client, exitBlocked, "no_published_port",
			"This project publishes no database port, so nothing on this machine can reach it",
			binaryName()+" tunnel db, and then "+binaryName()+" "+client+" --port=<the port it prints>")
	}

	if jsonOutput {
		return document(stdout, stderr, client, reach)
	}

	if printOnly {
		fmt.Fprintf(stdout, "%s\n", reach.URL)

		return exitOK
	}

	name := clientName(client)
	command, arguments := clientCommand(client, reach.URL)

	if err := openWith(command, arguments...); err == nil {
		// print_code is BROWN in the shell, which is the same sequence as its YELLOW
		fmt.Fprint(stdout, good("Opened "+name+" on ")+warning(description.Project.Name)+"\n")

		return exitOK
	}

	//
	// What the person needed was the connection. A command that says "not installed" and nothing
	// else has wasted the trip — which is also why this is not a failure.
	//
	fmt.Fprint(stderr, warning(name+" could not be opened; it may not be installed")+"\n")
	fmt.Fprint(stdout, code(reset)+"  The connection:\n\n    "+warning(reach.URL)+"\n\n")

	return exitOK
}

// serviceRunning reports whether the description has the service's container running.
func serviceRunning(description core.Description, service string) bool {
	for _, candidate := range description.Services {
		if candidate.Name == service {
			return candidate.State == "running"
		}
	}

	return false
}

// connectionOf builds the connection from the description, with the port forced when one was
// given. The credentials fall back to "magento" the way the shell's `jq // "magento"` did.
func connectionOf(description core.Description, forcedPort string) connection {
	reach := connection{Host: "127.0.0.1", User: "magento", Database: "magento"}
	password := "magento"

	if credentials := description.Credentials; credentials != nil {
		reach.User = orMagento(credentials.Database.User)
		password = orMagento(credentials.Database.Password)
		reach.Database = orMagento(credentials.Database.Name)
	}

	reach.Port = forcedPort
	if reach.Port == "" {
		reach.Port = publishedPort(description, "db", "3306")
	}

	// No escaping, as hm_db_url did none
	reach.URL = fmt.Sprintf("mysql://%s:%s@%s:%s/%s", reach.User, password, reach.Host, reach.Port, reach.Database)

	return reach
}

func orMagento(value string) string {
	if value == "" {
		return "magento"
	}

	return value
}

// publishedPort is the host port the service publishes for the container's target port, or ""
// when it publishes none. The description lists each port as "<published>-><target>", and like
// the shell's jq the first port with that target decides, published or not.
func publishedPort(description core.Description, service, target string) string {
	for _, candidate := range description.Services {
		if candidate.Name != service {
			continue
		}

		for _, port := range candidate.Ports {
			published, wanted, found := strings.Cut(port, "->")
			if found && wanted == target {
				return published
			}
		}
	}

	return ""
}

// clientCommand is the command that opens each client. TablePlus and Sequel Ace both take a
// `mysql://` URL; on Linux the URL is handed to the desktop, which is what `xdg-open` is for.
func clientCommand(client, url string) (string, []string) {
	if goos == "darwin" {
		switch client {
		case "tableplus":
			return "open", []string{"-a", "TablePlus", url}
		case "sequelace":
			return "open", []string{"-a", "Sequel Ace", url}
		case "dbeaver":
			return "open", []string{"-a", "DBeaver", url}
		}
	}

	if client == "dbeaver" {
		return "dbeaver", []string{url}
	}

	return "xdg-open", []string{url}
}

func clientName(client string) string {
	switch client {
	case "tableplus":
		return "TablePlus"
	case "sequelace":
		return "Sequel Ace"
	case "dbeaver":
		return "DBeaver"
	}

	return client
}
