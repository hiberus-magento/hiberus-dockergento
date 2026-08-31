package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/composecfg"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/composelib"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/dockerd"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/legacy"
	"github.com/hiberus-magento/hiberus-dockergento/internal/app"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// operatorFor wires the everyday operations to the machine.
func operatorFor(project core.Project, stdout, stderr io.Writer, jsonOutput bool) app.Operator {
	return app.Operator{
		Orchestrator: composelib.Orchestrator{Environment: environmentFor(project)},
		Engine:       dockerd.Engine{},
		Legacy:       legacy.Runner{},
		Platform:     machineName(),
		Binary:       binaryName(),
		Workdir:      property(project, "WORKDIR_PHP"),

		// In JSON mode stdout carries the document, so anything decorative goes to stderr —
		// otherwise a program reading the output finds a sentence in the middle of it
		Announce: func(message string) {
			where := stdout
			if jsonOutput {
				where = stderr
			}

			fmt.Fprint(where, good(message))
		},
	}
}

func start(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	stopOthers := false
	services := []string{}

	for _, argument := range args {
		switch argument {
		case "-s":
			stopOthers = true
		default:
			if strings.HasPrefix(argument, "-") {
				return failure(stderr, jsonOutput, "start", exitUsage, "invalid_argument",
					fmt.Sprintf("Unknown option: %s", argument), binaryName()+" start [-s] [service...]")
			}

			services = append(services, argument)
		}
	}

	project, code := projectOr(stderr, jsonOutput, "start")
	if code != 0 {
		return code
	}

	operator := operatorFor(project, stdout, stderr, jsonOutput)

	return report(stderr, jsonOutput, "start",
		operator.Start(project, composeFilesFor(project), services, stopOthers, usesProxy(project)))
}

func stop(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	snapshot := false
	services := []string{}

	for _, argument := range args {
		switch argument {
		case "--snapshot":
			snapshot = true
		default:
			services = append(services, argument)
		}
	}

	project, code := projectOr(stderr, jsonOutput, "stop")
	if code != 0 {
		return code
	}

	operator := operatorFor(project, stdout, stderr, jsonOutput)

	return report(stderr, jsonOutput, "stop",
		operator.Stop(project, composeFilesFor(project), services, snapshot))
}

func restart(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	services := []string{}

	for _, argument := range args {
		if strings.HasPrefix(argument, "-") {
			return failure(stderr, jsonOutput, "restart", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), binaryName()+" restart [service...]")
		}

		services = append(services, argument)
	}

	project, code := projectOr(stderr, jsonOutput, "restart")
	if code != 0 {
		return code
	}

	operator := operatorFor(project, stdout, stderr, jsonOutput)

	return report(stderr, jsonOutput, "restart",
		operator.Restart(project, composeFilesFor(project), services, usesProxy(project)))
}

// logs writes what the services are saying.
//
// The options are enumerated rather than passed through. The shell implementation forwarded
// whatever it was given and had to know separately which flags take a value; getting that list
// wrong meant `hm logs --tail 3` read the 3 as a service name and refused to run.
func logs(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options := core.LogOptions{}
	services := []string{}

	// The options that take a separate value have to be known by name: without that list,
	// `hm logs --tail 3` reads the 3 as a service and refuses to run
	takesValue := map[string]bool{"--tail": true, "-n": true, "--since": true, "--until": true, "--index": true}

	for at := 0; at < len(args); at++ {
		argument := args[at]

		if takesValue[argument] {
			if at+1 >= len(args) {
				return failure(stderr, jsonOutput, "logs", exitUsage, "missing_value",
					argument+" needs a value", binaryName()+" logs --tail 100")
			}

			at++
			value := args[at]

			switch argument {
			case "--tail", "-n":
				options.Tail = value
			case "--since":
				options.Since = value
			case "--until":
				options.Until = value
			case "--index":
				index, err := strconv.Atoi(value)
				if err != nil {
					return failure(stderr, jsonOutput, "logs", exitUsage, "invalid_argument",
						"--index needs a number", binaryName()+" logs --index 1")
				}

				options.Index = index
			}

			continue
		}

		switch argument {
		case "-f", "--follow":
			options.Follow = true
		case "-t", "--timestamps":
			options.Timestamps = true
		case "--no-color":
			options.NoColor = true
		case "--no-log-prefix":
			options.NoPrefix = true
		default:
			if strings.HasPrefix(argument, "-") {
				return failure(stderr, jsonOutput, "logs", exitUsage, "invalid_argument",
					fmt.Sprintf("Unknown option: %s", argument), binaryName()+" logs [-f] [service...]")
			}

			services = append(services, argument)
		}
	}

	project, code := projectOr(stderr, jsonOutput, "logs")
	if code != 0 {
		return code
	}

	files := composeFilesFor(project)

	//
	// Compose answers a wrong service name with an error about YAML files. The configuration is
	// already here, so the message can say what this project actually has.
	//
	// Only when a service was named: `hm logs` on its own has nothing to validate.
	//
	if len(services) > 0 {
		if code := checkServices(project, files, services, stderr, jsonOutput, "logs"); code != 0 {
			return code
		}
	}

	operator := operatorFor(project, stdout, stderr, jsonOutput)

	return report(stderr, jsonOutput, "logs",
		operator.Orchestrator.Logs(project, files, services, options))
}

// exec runs something inside the php container, which is the one people mean.
func execute(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	user := ""

	if len(args) > 0 && args[0] == "-r" {
		user = "root"
		args = args[1:]
	}

	if len(args) == 0 {
		return failure(stderr, jsonOutput, "exec", exitUsage, "missing_command",
			"There is no command to run", binaryName()+" exec ls -lah")
	}

	project, code := projectOr(stderr, jsonOutput, "exec")
	if code != 0 {
		return code
	}

	operator := operatorFor(project, stdout, stderr, jsonOutput)

	status, err := operator.Orchestrator.Exec(project, composeFilesFor(project), phpService,
		args, terminalOptions(user))
	if status != 0 {
		return status
	}

	return report(stderr, jsonOutput, "exec", err)
}

// checkServices refuses a service this project does not have, and says which it does.
func checkServices(project core.Project, files core.ComposeFiles, wanted []string,
	stderr io.Writer, jsonOutput bool, command string) int {
	configuration, err := (composecfg.Loader{Environment: environmentFor(project)}).
		Load(project.Root, project.Name, files.Load)
	if err != nil {
		return 0
	}

	available := make([]string, 0, len(configuration.Services))
	for _, service := range configuration.Services {
		available = append(available, service.Name)
	}

	for _, service := range wanted {
		if contains(available, service) {
			continue
		}

		return failure(stderr, jsonOutput, command, exitService, "unknown_service",
			fmt.Sprintf("This project has no service called '%s'", service),
			fmt.Sprintf("%s %s %s", binaryName(), command, strings.Join(available, " ")))
	}

	return 0
}

// projectOr resolves the project or reports why it could not.
func projectOr(stderr io.Writer, jsonOutput bool, command string) (core.Project, int) {
	project, err := resolveProject()
	if err != nil || project.Name == "" {
		return core.Project{}, failure(stderr, jsonOutput, command, exitProject, "project_not_configured",
			"This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid",
			binaryName()+" setup")
	}

	return project, 0
}

// report turns what an operation returned into the exit code its caller branches on.
func report(stderr io.Writer, jsonOutput bool, command string, err error) int {
	if err == nil {
		return exitOK
	}

	var refusal core.Refusal
	if asRefusal(err, &refusal) {
		return failure(stderr, jsonOutput, command, refusal.Code, refusal.Kind, refusal.Message, refusal.Hint)
	}

	return failure(stderr, jsonOutput, command, exitDocker, "docker_failed", err.Error(), "")
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

// usesProxy reports whether this project is routed through the one proxy on the machine.
//
// Never for a branch environment: it does not carry the proxy overlay — that overlay claims the
// main environment's address — so starting the proxy for it would achieve nothing, and could
// refuse the start outright when another environment happens to hold port 80.
func usesProxy(project core.Project) bool {
	return project.Worktree == nil && truthy(property(project, "USE_PROXY"))
}

func truthy(value string) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "1":
		return true
	}

	return false
}
