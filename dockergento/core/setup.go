package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//
// Creating a project's environment: what `setup` was told to do, and what it writes.
//
// Reading the instruction is kept apart from carrying it out, and not for tidiness. Creating an
// environment takes minutes and a working Docker; deciding what `--db-dump=./x.sql
// --clean-install` means takes neither, and that is where the mistakes were — a dump path that
// did not exist used to be a warning, after which the command carried on and asked the question
// interactively, so an automated bootstrap with a wrong path hung instead of failing.
//

// SetupOptions is what setup was told.
type SetupOptions struct {
	// Dump is a database to import instead of installing Magento from scratch.
	Dump string

	// Install asks for a clean Magento installation.
	Install bool

	// ProjectName, Domain and Root are the three things a project is: what its containers are
	// called, what address it answers on, and where its code is.
	ProjectName string
	Domain      string
	Root        string

	// Force regenerates the compose files even when they are already there and ours.
	Force bool

	// UseDefault answers every question with its suggestion.
	UseDefault bool

	// Mail is which mail catcher this project runs.
	Mail string
}

// ParseSetup reads the instruction, and refuses one it does not understand before anything is
// created.
//
// `--clean-install` and `--db-dump` are Warden's names for the same two things. Half a department
// has used it, and refusing the word somebody typed in order to be tidy is being tidy at their
// expense.
func ParseSetup(args []string, binary string) (SetupOptions, error) {
	options := SetupOptions{}

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
		case argument == "-i" || argument == "--install" || argument == "--clean-install":
			options.Install = true
		case argument == "-D" || argument == "--dump" || argument == "--db-dump":
			options.Dump = value()
		case strings.HasPrefix(argument, "--dump="):
			options.Dump = strings.TrimPrefix(argument, "--dump=")
		case strings.HasPrefix(argument, "--db-dump="):
			options.Dump = strings.TrimPrefix(argument, "--db-dump=")
		case argument == "-p" || argument == "--project-name":
			options.ProjectName = value()
		case strings.HasPrefix(argument, "--project-name="):
			options.ProjectName = strings.TrimPrefix(argument, "--project-name=")
		case argument == "-d" || argument == "--domain":
			options.Domain = value()
		case strings.HasPrefix(argument, "--domain="):
			options.Domain = strings.TrimPrefix(argument, "--domain=")
		case argument == "-r" || argument == "--root-directory":
			options.Root = value()
		case strings.HasPrefix(argument, "--root-directory="):
			options.Root = strings.TrimPrefix(argument, "--root-directory=")
		case argument == "--mail":
			options.Mail = value()
		case strings.HasPrefix(argument, "--mail="):
			options.Mail = strings.TrimPrefix(argument, "--mail=")
		case argument == "-f" || argument == "--force":
			options.Force = true
		case argument == "-u" || argument == "--use-default":
			options.UseDefault = true
		case strings.HasPrefix(argument, "-"):
			return options, usage("invalid_argument", "Unknown option: "+argument,
				binary+" setup --domain=shop.test --clean-install")
		default:
			return options, usage("unexpected_argument",
				fmt.Sprintf("'%s' is not something setup takes", argument), binary+" setup --help")
		}
	}

	return options, nil
}

// MagentoRoot is a directory answer turned into what the compose files can use.
//
// Relative and with no trailing slash, because it is written into a bind mount: `app/` and `./app`
// are the same directory to a person and two different strings in a generated file.
func MagentoRoot(answer, current string) string {
	answer = strings.TrimSuffix(answer, "/")

	if answer != "" && answer != "." &&
		!strings.HasPrefix(answer, "./") && !strings.HasPrefix(answer, "/") {
		answer = "./" + answer
	}

	if answer == "" {
		return current
	}

	return answer
}

//
// The compose file a project runs on is generated from a template: the services are always the
// same, and what changes is which image each one is.
//

// ServiceImage is the image a service runs, from what the requirements say about it.
//
// A value with a colon in it is already an image somebody named. Anything else is a version of one
// of ours, which is the common case and the reason the file does not have to spell it out.
func ServiceImage(service, requirement string) string {
	if strings.Contains(requirement, ":") {
		return requirement
	}

	return "hiberusmagento/" + service + ":" + requirement
}

// RenderCompose fills the template in: one substitution per service, and two for the mail catcher.
//
// The template stays a list of replacements with no logic in it. The two variable points — which
// mail service, and which image — are two more markers, because teaching the template to decide
// would mean turning the generator into something that evaluates conditions, which is a lot of
// machinery for one choice.
func RenderCompose(template string, requirements map[string]string, mail string) (string, error) {
	if mail == "" {
		mail = "mailhog"
	}

	if mail != "mailhog" && mail != "mailpit" {
		return "", Refusal{
			Kind:    "unknown_mail_service",
			Code:    2,
			Message: fmt.Sprintf("'%s' is not a mail catcher this tool knows about", mail),
			Hint:    "Set MAIL_SERVICE to mailhog or mailpit",
		}
	}

	// In a settled order, so that a service whose marker is a prefix of another's cannot depend
	// on which one happened to be replaced first
	services := make([]string, 0, len(requirements))
	for service := range requirements {
		services = append(services, service)
	}

	sort.Strings(services)

	written := template

	for _, service := range services {
		written = strings.ReplaceAll(written,
			"<"+service+"_version>", ServiceImage(service, requirements[service]))
	}

	image, declared := requirements[mail]
	if !declared || image == "" {
		return "", Refusal{
			Kind:    "mail_service_unavailable",
			Code:    4,
			Message: fmt.Sprintf("No %s image is defined for this Magento version", mail),
			Hint:    "Set MAIL_SERVICE to mailhog in config/docker/properties.json",
		}
	}

	written = strings.ReplaceAll(written, "<mail_service>", mail)
	written = strings.ReplaceAll(written, "<mail_version>", ServiceImage(mail, image))

	return written, nil
}

// ComposeVersionLine is the `version:` key a compose file used to need.
//
// Compose stopped wanting it in 2.25 and warns about it, so it is written only for the versions
// that still expect it.
func ComposeVersionLine(composeVersion string) string {
	if composeVersion != "" && ComposeAtLeast(composeVersion, "2.25") {
		return ""
	}

	return "version: \"3.7\"\n"
}

// BindMounts is the list of a repository's own top-level entries mounted into the container, as
// the mac overlay writes them.
//
// Everything the repository tracks that is not already mounted and is not one of Magento's own
// files: on macOS the code is copied into a volume for speed, and what is left outside would be
// invisible to PHP.
func BindMounts(entries []string, magentoDir, suffix string, skip map[string]bool, existing string) string {
	mounts := []string{}

	for _, entry := range entries {
		if entry == "" || skip[entry] || entry == "vendor" {
			continue
		}

		if magentoDir == entry || strings.HasPrefix(magentoDir, entry+"/") {
			continue
		}

		// Already mounted by the template, which mounts the ones every Magento has. Adding a
		// second entry for the same path is a mount declared twice, and Compose takes the last
		mount := fmt.Sprintf("%s/%s:/var/www/html/%s", magentoDir, entry, entry)
		if strings.Contains(existing, mount) {
			continue
		}

		mounts = append(mounts, "- "+mount+suffix)
	}

	if len(mounts) == 0 {
		return ""
	}

	return strings.Join(mounts, "\n      ") + "\n"
}

// DeriveProjectName is the name Docker Compose would give a directory.
//
// The rule is Compose's, measured rather than assumed: lowercase, keep only [a-z0-9_-] — accented
// characters are dropped, not transliterated — and trim leading dashes and underscores. If nothing
// admissible is left there is no name, and the tool asks for one.
func DeriveProjectName(dir string) string {
	name := dir
	if at := strings.LastIndex(name, "/"); at >= 0 {
		name = name[at+1:]
	}

	kept := strings.Builder{}

	for _, character := range strings.ToLower(name) {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9',
			character == '_', character == '-':
			kept.WriteRune(character)
		}
	}

	return strings.TrimLeft(kept.String(), "-_")
}

// ExpandHome turns a path somebody typed with a tilde into one the filesystem understands.
func ExpandHome(path, home string) string {
	if home == "" || !strings.HasPrefix(path, "~/") {
		return path
	}

	return home + path[1:]
}

//
// The tables this reads are data files of the tool's own, so a malformed one is a broken
// installation rather than something a project did.
//

// ReadRequirements is the table of what each Magento needs, keyed by the version a release
// resolves to.
func ReadRequirements(contents string) (map[string]map[string]string, error) {
	table := map[string]map[string]string{}

	if contents == "" {
		return table, Refusal{
			Kind:    "requirements_missing",
			Code:    1,
			Message: "The table of supported versions is missing from this installation",
			Hint:    "Reinstall the tool",
		}
	}

	return table, json.Unmarshal([]byte(contents), &table)
}

// ReadEquivalents maps a Magento release to the row of the table it uses. Forty patch releases
// share one set of images, and a table with a row per patch would be forty rows to keep in step.
func ReadEquivalents(contents string) (map[string]string, error) {
	table := map[string]string{}

	if contents == "" {
		return table, nil
	}

	return table, json.Unmarshal([]byte(contents), &table)
}

// ReadFlags is a table of names to yes-or-no, which is the shape of the list of files Magento
// brings itself.
func ReadFlags(contents string) map[string]bool {
	table := map[string]bool{}

	if contents == "" {
		return table
	}

	json.Unmarshal([]byte(contents), &table) //nolint:errcheck

	return table
}

// Truthy reads the words a properties file uses for yes.
func Truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true
	}

	return false
}
