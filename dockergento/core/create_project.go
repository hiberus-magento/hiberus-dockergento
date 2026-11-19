package core

import (
	"fmt"
	"strconv"
	"strings"
)

//
// Creating a new Magento project: what `create-project` was told to do.
//
// Read whole before anything happens, like setup's instruction and for the same reason: creating a
// project downloads Magento and takes minutes, and an edition nobody sells is better refused before
// the first container starts than after.
//

// CreateProjectOptions is what create-project was told.
type CreateProjectOptions struct {
	// ProjectName and Domain are what the containers are called and the address they answer on.
	// Naming the project suggests its domain, NAME.local, as the shell implementation did.
	ProjectName string
	Domain      string

	// Edition is which Magento: community or enterprise.
	Edition string

	// Version is the Magento release to create.
	Version string

	// Root is where the code lives, relative to the project.
	Root string

	// UseDefault answers every question with its suggestion: the directory's name, community, the
	// latest supported release and the project's own directory.
	UseDefault bool
}

// Editions are the Magento editions there is a project template for.
var Editions = []string{"community", "enterprise"}

// ParseCreateProject reads the instruction, and refuses one it does not understand before anything
// is created.
//
// The short options are the shell implementation's, and the contract. The long ones are setup's
// names for the same things, so that somebody who learned one command is not wrong in the other.
func ParseCreateProject(args []string, binary string) (CreateProjectOptions, error) {
	options := CreateProjectOptions{}

	usage := func(kind, message, hint string) error {
		return Refusal{Kind: kind, Code: 2, Message: message, Hint: hint}
	}

	for at := 0; at < len(args); at++ {
		argument := args[at]

		value := func() string {
			if at+1 < len(args) {
				at++

				return args[at]
			}

			return ""
		}

		switch {
		case argument == "-p" || argument == "--project-name":
			options.ProjectName = value()
		case strings.HasPrefix(argument, "--project-name="):
			options.ProjectName = strings.TrimPrefix(argument, "--project-name=")
		case argument == "-e" || argument == "--edition":
			options.Edition = value()
		case strings.HasPrefix(argument, "--edition="):
			options.Edition = strings.TrimPrefix(argument, "--edition=")
		case argument == "-v" || argument == "--version":
			options.Version = value()
		case strings.HasPrefix(argument, "--version="):
			options.Version = strings.TrimPrefix(argument, "--version=")
		case argument == "-r" || argument == "--root-directory":
			options.Root = value()
		case strings.HasPrefix(argument, "--root-directory="):
			options.Root = strings.TrimPrefix(argument, "--root-directory=")
		case argument == "-u" || argument == "--use-default":
			options.UseDefault = true
		case strings.HasPrefix(argument, "-"):
			return options, usage("invalid_argument", "Unknown option: "+argument,
				binary+" create-project -p shop -e community -v 2.4.7")
		default:
			return options, usage("unexpected_argument",
				fmt.Sprintf("'%s' is not something create-project takes", argument),
				binary+" create-project --help")
		}
	}

	// The domain follows the name it was given, whatever order the two options came in: the shell
	// implementation set both from -p, and -u set the domain from the name again
	if options.ProjectName != "" {
		options.Domain = options.ProjectName + ".local"
	}

	// What -u means for the two answers that need nothing but the instruction. The version it
	// means needs the table of supported releases, and is settled where that table is read. An
	// explicit option wins over the default either way round, as it did with getopts
	if options.UseDefault {
		options.Edition = first(options.Edition, "community")
		options.Root = first(options.Root, ".")
	}

	if options.Edition != "" && !knownEdition(options.Edition) {
		return options, usage("invalid_edition",
			fmt.Sprintf("Edition '%s' is not available (%s)", options.Edition, strings.Join(Editions, " or ")),
			binary+" create-project -e community")
	}

	return options, nil
}

func knownEdition(edition string) bool {
	for _, known := range Editions {
		if edition == known {
			return true
		}
	}

	return false
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

// LatestVersion is the newest of a list of Magento releases, by release order.
//
// Not the last one in alphabetical order, which is what the shell implementation took: 2.4.10
// sorts before 2.4.9 as text, so the day the table gains it the "latest" would quietly go back a
// release. Patch releases come after the release they patch, and -p10 after -p9.
func LatestVersion(versions []string) string {
	latest := ""

	for _, version := range versions {
		if latest == "" || releaseAfter(version, latest) {
			latest = version
		}
	}

	return latest
}

// releaseAfter reports whether one Magento release is newer than another.
func releaseAfter(one, other string) bool {
	left, right := releaseNumbers(one), releaseNumbers(other)

	for at := 0; at < len(left) || at < len(right); at++ {
		a, b := 0, 0

		if at < len(left) {
			a = left[at]
		}

		if at < len(right) {
			b = right[at]
		}

		if a != b {
			return a > b
		}
	}

	return false
}

// releaseNumbers is a release as numbers: 2.4.7-p3 is 2, 4, 7, 3 and 2.4.7 is 2, 4, 7, 0, which
// puts a patch release after the one it patches.
func releaseNumbers(version string) []int {
	base, patch, _ := strings.Cut(strings.TrimSpace(version), "-")

	numbers := []int{}

	for _, part := range strings.Split(base, ".") {
		value, _ := strconv.Atoi(part)
		numbers = append(numbers, value)
	}

	// Always the same length before the patch number, so that 2.4 and 2.4.0-p1 compare position by
	// position
	for len(numbers) < 3 {
		numbers = append(numbers, 0)
	}

	value, _ := strconv.Atoi(strings.TrimLeft(patch, "p"))

	return append(numbers, value)
}
