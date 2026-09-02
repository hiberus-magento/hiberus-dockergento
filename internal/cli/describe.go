package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func describe(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	withSecrets := false

	for _, argument := range args {
		switch argument {
		case "--with-secrets":
			withSecrets = true
		default:
			return failure(stderr, jsonOutput, "describe", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), "hm describe --help")
		}
	}

	description, err := engine(stdout, stderr, jsonOutput).Describe(here(), withSecrets)
	if err != nil {
		return failure(stderr, jsonOutput, "describe", exitProject, "project_not_configured",
			fmt.Sprintf("This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid: %s", err),
			"hm setup")
	}

	if jsonOutput {
		document, err := json.MarshalIndent(contract.Success("describe", description), "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err)

			return exitError
		}

		fmt.Fprintf(stdout, "%s\n", document)

		return 0
	}

	describeAsText(description, stdout)

	return 0
}

func describeAsText(description core.Description, stdout io.Writer) {
	// A rule above and below the name, painted as one block with a single reset at the end —
	// which is how the shell implementation draws it, trailing newline included
	fmt.Fprint(stdout, header(rule+"\n"+description.Project.Name+"\n"+rule+"\n"))

	fmt.Fprintf(stdout, "\n%s\n", section("Environment"))
	fmt.Fprintf(stdout, "   %-14s %s\n", "status", orDash(description.Project.Status))
	fmt.Fprintf(stdout, "   %-14s %s\n", "domain", orDash(description.Project.Domain))
	fmt.Fprintf(stdout, "   %-14s %s\n", "worktree", orDash(description.Project.Worktree))
	fmt.Fprintf(stdout, "   %-14s %s\n", "root", description.Project.Root)

	fmt.Fprintf(stdout, "\n%s\n", section("URLs"))
	for _, url := range []struct{ name, value string }{
		{"base", description.Project.URLs.Base},
		{"admin", description.Project.URLs.Admin},
		{"mail", description.Project.URLs.Mail},
		{"mailhog", description.Project.URLs.Mailhog},
		{"rabbitmq", description.Project.URLs.RabbitMQ},
		{"search", description.Project.URLs.Search},
	} {
		if url.value == "" {
			continue
		}

		fmt.Fprintf(stdout, "   %-14s %s\n", url.name, link(url.value))
	}

	fmt.Fprintf(stdout, "\n%s\n", section("Magento"))
	fmt.Fprintf(stdout, "   %-14s %s\n", "version", orUnknown(description.Magento.Version))
	fmt.Fprintf(stdout, "   %-14s %s\n", "mode", orUnknown(description.Magento.Mode))
	fmt.Fprintf(stdout, "   %-14s %s\n", "xdebug", description.Tooling.Xdebug)

	fmt.Fprintf(stdout, "\n%s\n", section("Services"))
	for _, service := range description.Services {
		fmt.Fprintf(stdout, "   %-14s %-10s %s\n", service.Name, service.State, service.Image)
	}

	fmt.Fprintf(stdout, "\n%s\n", section("Paths"))
	fmt.Fprintf(stdout, "   %-14s %s\n", "magento dir", description.Paths.MagentoDir)
	fmt.Fprintf(stdout, "   %-14s %s\n", "mounts", description.Paths.Strategy)

	if description.Credentials != nil {
		fmt.Fprintf(stdout, "\n%s\n", warning("Credentials"))
		fmt.Fprintf(stdout, "   %-14s %s\n", "db name", description.Credentials.Database.Name)
		fmt.Fprintf(stdout, "   %-14s %s\n", "db user", description.Credentials.Database.User)
		fmt.Fprintf(stdout, "   %-14s %s\n", "db password", description.Credentials.Database.Password)
		fmt.Fprintf(stdout, "   %-14s %s\n", "db root_password", description.Credentials.Database.RootPassword)
	}

	fmt.Fprintf(stdout, "\n")
}

func orUnknown(value string) string {
	if value == "" {
		return "unknown"
	}

	return value
}
