package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

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

	if _, code := projectOr(stderr, jsonOutput, "start"); code != 0 {
		return code
	}

	return report(stderr, jsonOutput, "start", engine(stdout, stderr, jsonOutput).
		Start(here(), dockergento.StartOptions{Services: services, StopOthers: stopOthers}))
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

	if _, code := projectOr(stderr, jsonOutput, "stop"); code != 0 {
		return code
	}

	return report(stderr, jsonOutput, "stop", engine(stdout, stderr, jsonOutput).
		Stop(here(), services, snapshot))
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

	if _, code := projectOr(stderr, jsonOutput, "restart"); code != 0 {
		return code
	}

	return report(stderr, jsonOutput, "restart", engine(stdout, stderr, jsonOutput).
		Restart(here(), services))
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

	//
	// Compose answers a wrong service name with an error about YAML files. The configuration is
	// already here, so the message can say what this project actually has.
	//
	// Only when a service was named: `hm logs` on its own has nothing to validate.
	//
	if len(services) > 0 {
		if code := checkServices(project, services, stderr, jsonOutput, "logs"); code != 0 {
			return code
		}
	}

	return report(stderr, jsonOutput, "logs", engine(stdout, stderr, jsonOutput).
		Logs(here(), services, options))
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

	if _, code := projectOr(stderr, jsonOutput, "exec"); code != 0 {
		return code
	}

	status, err := engine(stdout, stderr, jsonOutput).
		Exec(here(), phpService, args, terminalOptions(user))
	if status != 0 {
		return status
	}

	return report(stderr, jsonOutput, "exec", err)
}

// checkServices refuses a service this project does not have, and says which it does.
func checkServices(project core.Project, wanted []string,
	stderr io.Writer, jsonOutput bool, command string) int {
	configuration, err := engine(nil, nil, jsonOutput).Configuration(project)
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
