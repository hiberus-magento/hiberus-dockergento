package cli

import (
	"fmt"
	"io"
	"strings"
)

//
// The commands that are one thing run in one container.
//
// They are wrappers, and that is the whole point of them: knowing that the unit tests are run with
// this phpunit and that configuration, and that clearing generated code means these seven
// directories, is what the tool is for. What they must not do is differ from each other in how
// they reach the container.
//

// purge clears everything Magento generates and can generate again.
//
// The list is exact rather than "var and generated": `var/log` is in neither, because a developer
// looking for what went wrong an hour ago should still find it.
var generated = []string{
	"var/cache/*", "generated/*", "pub/static/*", "var/view_preprocessed/*",
	"var/page_cache/*", "var/generation/*", "dev/tests/integration/tmp/*",
}

func purge(stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "purge")
	if code != 0 {
		return code
	}

	return inside(project, []string{"sh", "-c", "rm -rf " + strings.Join(generated, " ")},
		"", stdout, stderr, jsonOutput, "purge")
}

// npm runs the front-end's package manager where the front end is built.
func npm(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "npm")
	if code != 0 {
		return code
	}

	return inside(project, append([]string{"npm"}, args...), "", stdout, stderr, jsonOutput, "npm")
}

// magerun is n98-magerun, which is in the image because half of what people ask Magento to do is
// quicker through it.
func magerun(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "n98-magerun")
	if code != 0 {
		return code
	}

	return inside(project, []string{"bash", "-c", "n98-magerun " + strings.Join(args, " ")},
		"", stdout, stderr, jsonOutput, "n98-magerun")
}

// tests runs one of the two suites, with the configuration each of them needs.
//
// The unit suite runs from the project root and the integration one from its own directory, which
// is not a preference: the integration configuration resolves paths relative to itself.
func tests(kind string, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "test-"+kind)
	if code != 0 {
		return code
	}

	binDir := engine(stdout, stderr, jsonOutput).Property(project, "BIN_DIR")
	if binDir == "" {
		binDir = "./vendor/bin"
	}

	command := binDir + "/phpunit --config ./dev/tests/unit/phpunit.xml.dist"

	if kind == "integration" {
		workdir := engine(stdout, stderr, jsonOutput).Property(project, "WORKDIR_PHP")
		if workdir == "" {
			workdir = "/var/www/html"
		}

		command = fmt.Sprintf("cd ./dev/tests/integration && %s/%s/phpunit --config phpunit.xml",
			workdir, binDir)
	}

	if len(args) > 0 {
		command += " " + strings.Join(args, " ")
	}

	return inside(project, []string{"sh", "-c", command}, "", stdout, stderr, jsonOutput, "test-"+kind)
}

// dump writes the project's database to a file.
func dump(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) == 0 || args[0] == "" {
		return failure(stderr, jsonOutput, "mysqldump", exitUsage, "missing_path",
			"Where should the dump be written?", binaryName()+" mysqldump /path/to/dump.sql")
	}

	if _, code := projectOr(stderr, jsonOutput, "mysqldump"); code != 0 {
		return code
	}

	if err := engine(stdout, stderr, jsonOutput).Dump(here(), args[0]); err != nil {
		return report(stderr, jsonOutput, "mysqldump", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "mysqldump", map[string]any{"path": args[0]})
	}

	fmt.Fprint(stdout, good("Written to "))
	fmt.Fprint(stdout, warning(args[0]))
	fmt.Fprint(stdout, "\n")

	return exitOK
}

//
// Varnish, on and off.
//
// Two things have to agree: the VCL, which decides whether Varnish passes everything through, and
// Magento's full page cache, which decides whether it is asked to. Changing one and not the other
// is a page cache that looks enabled and caches nothing, which is the kind of thing that is found
// a week later while measuring something else.
//

const varnishService = "varnish"

// skipMarker is the line in the VCL that is commented in and out. It carries its own comment so
// that a person reading the file can see which line this tool touches.
const skipMarker = "#skip-varnish"

func varnish(on bool, stdout, stderr io.Writer, jsonOutput bool) int {
	command := "varnish-off"
	edit := `s/#\+return(pass); ` + skipMarker + `/ return(pass); ` + skipMarker + `/g`
	cache := "cache:disable"

	if on {
		command = "varnish-on"
		edit = `s/^[^#]\+return(pass); ` + skipMarker + `/#return(pass); ` + skipMarker + `/g`
		cache = "cache:enable"
	}

	project, code := projectOr(stderr, jsonOutput, command)
	if code != 0 {
		return code
	}

	engine := engine(stdout, stderr, jsonOutput)

	status, err := engine.Exec(project.Root, varnishService,
		[]string{"sed", "-i", edit, "/etc/varnish/default.vcl"},
		terminalOptions("root"))
	if err != nil || status != 0 {
		return report(stderr, jsonOutput, command, err)
	}

	if err := engine.Restart(project.Root, []string{varnishService}); err != nil {
		return report(stderr, jsonOutput, command, err)
	}

	status, err = engine.Exec(project.Root, phpService,
		[]string{"bin/magento", cache, "full_page"}, terminalOptions(""))
	if err != nil || status != 0 {
		return report(stderr, jsonOutput, command, err)
	}

	//
	// Turning it off leaves pages cached by it behind, and they are served until they expire. So
	// what it generated goes with it.
	//
	if !on {
		if code := purge(stdout, stderr, jsonOutput); code != 0 {
			return code
		}

		if code := magento([]string{"cache:clean"}, stdout, stderr, jsonOutput); code != 0 {
			return code
		}
	}

	if jsonOutput {
		return document(stdout, stderr, command, map[string]any{"varnish": on})
	}

	if on {
		fmt.Fprint(stdout, good("Varnish cache enabled!\n"))

		return exitOK
	}

	fmt.Fprint(stdout, good("Varnish cache disabled!\n"))

	return exitOK
}

//
// Moving a project's files between this machine and its container.
//
// Only on macOS is this something anybody has to think about: there the code lives in a volume
// rather than a mount, which is what makes PHP fast enough to work in, and the price is that the
// two sides are two places.
//

func copyInto(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) == 0 {
		return failure(stderr, jsonOutput, "copy-to-container", exitUsage, "missing_path",
			"What should be copied into the container?",
			binaryName()+" copy-to-container app/code")
	}

	if _, code := projectOr(stderr, jsonOutput, "copy-to-container"); code != 0 {
		return code
	}

	all := args[0] == "--all"

	err := engine(stdout, stderr, jsonOutput).CopyInto(here(), args, all)
	if err != nil {
		return report(stderr, jsonOutput, "copy-to-container", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "copy-to-container", map[string]any{
			"copied": args, "all": all,
		})
	}

	return exitOK
}

func copyFrom(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) == 0 {
		return failure(stderr, jsonOutput, "copy-from-container", exitUsage, "missing_path",
			"What should be copied out of the container?",
			binaryName()+" copy-from-container generated")
	}

	if _, code := projectOr(stderr, jsonOutput, "copy-from-container"); code != 0 {
		return code
	}

	if err := engine(stdout, stderr, jsonOutput).CopyFrom(here(), args); err != nil {
		return report(stderr, jsonOutput, "copy-from-container", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "copy-from-container", map[string]any{"copied": args})
	}

	return exitOK
}

// setHost points a domain at this machine and tells Magento what it answers on.
//
// Two different things, and they fail differently: one needs the system password and touches a file
// every program on the machine reads; the other is a row in the project's database.
func setHost(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	domain := ""
	database := true
	remove := false

	for _, argument := range args {
		switch argument {
		case "--remove":
			remove = true
		case "--no-database":
			database = false
		default:
			if strings.HasPrefix(argument, "-") {
				return failure(stderr, jsonOutput, "set-host", exitUsage, "invalid_argument",
					"Unknown option: "+argument, binaryName()+" set-host shop.test")
			}

			domain = argument
		}
	}

	if remove {
		if err := engine(stdout, stderr, jsonOutput).RemoveHost(domain); err != nil {
			return report(stderr, jsonOutput, "set-host", err)
		}

		if jsonOutput {
			return document(stdout, stderr, "set-host", map[string]any{"removed": domain})
		}

		return exitOK
	}

	if _, code := projectOr(stderr, jsonOutput, "set-host"); code != 0 {
		return code
	}

	if err := engine(stdout, stderr, jsonOutput).SetHost(here(), domain, database); err != nil {
		return report(stderr, jsonOutput, "set-host", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "set-host", map[string]any{
			"domain": domain, "database": database,
		})
	}

	return exitOK
}
