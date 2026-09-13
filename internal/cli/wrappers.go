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

	binDir := newEngine(stdout, stderr, jsonOutput).Property(project, "BIN_DIR")
	if binDir == "" {
		binDir = "./vendor/bin"
	}

	command := binDir + "/phpunit --config ./dev/tests/unit/phpunit.xml.dist"

	if kind == "integration" {
		workdir := newEngine(stdout, stderr, jsonOutput).Property(project, "WORKDIR_PHP")
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

	if err := newEngine(stdout, stderr, jsonOutput).Dump(here(), args[0]); err != nil {
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

	engine := newEngine(stdout, stderr, jsonOutput)

	status, err := engine.Exec(project.Root, varnishService,
		[]string{"sed", "-i", edit, "/etc/varnish/default.vcl"},
		terminalOptions("root"))
	if status != 0 {
		return failure(stderr, jsonOutput, command, status, "command_failed",
			"The VCL edit failed in the varnish container", "")
	}
	if err != nil {
		return report(stderr, jsonOutput, command, err)
	}

	if err := engine.Restart(project.Root, []string{varnishService}); err != nil {
		return report(stderr, jsonOutput, command, err)
	}

	status, err = engine.Exec(project.Root, phpService,
		[]string{"bin/magento", cache, "full_page"}, terminalOptions(""))
	if status != 0 {
		return failure(stderr, jsonOutput, command, status, "command_failed",
			fmt.Sprintf("The %s failed in the phpfpm container", cache), "")
	}
	if err != nil {
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
// debug-off, alone.
//
// It shares phpService and terminalOptions with the varnish handlers above and needs no new
// engine surface: Exec and Compose are both already on commands.
//

func debugOff(stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "debug-off")
	if code != 0 {
		return code
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	// The shell script's own pattern carries a stray `\` before `z`
	// (`s/^\zend_extension/`), which GNU sed treats as the literal character rather than an
	// escape — the pattern already matched only `^zend_extension`. Dropped here on purpose,
	// not by oversight (proposal Decision 3).
	status, err := engine.Exec(project.Root, phpService,
		[]string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
			"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
		terminalOptions(""))
	if status != 0 {
		return failure(stderr, jsonOutput, "debug-off", status, "command_failed",
			"The xdebug ini edit failed in the phpfpm container", "")
	}
	if err != nil {
		return report(stderr, jsonOutput, "debug-off", err)
	}

	// Compose's own restart, not Operator.Restart: the edit lives inside the container
	// filesystem rather than in the compose file, a recreating restart is the wrong
	// primitive for it, and Operator.Restart refuses unconditionally from an unregistered
	// worktree (app/orchestrate.go:231-247) while Compose carries neither (:326-327) —
	// debug-off is not in hm_alters_environment, and this call must not become the first
	// thing that refuses it there (proposal Decision 1). The choice of method, not a
	// worktree-state assertion, is what proves this: see tasks.md's Phase 3.
	//
	// Its own (status, error) pair is handled exactly the way dockerCompose() handles the
	// identical pair (docker_compose.go:15-20), not the way the edit above is handled: a
	// non-zero status is returned as-is, unwrapped, because this is a Compose passthrough —
	// the same reasoning docker-compose's own case follows — while the edit above runs a
	// program inside a container, the same reasoning varnish's edit follows. Those are
	// different things this port keeps different rather than unifying by accident.
	status, err = engine.Compose(project.Root, []string{"restart", "phpfpm"})
	if err != nil {
		return report(stderr, jsonOutput, "debug-off", err)
	}
	if status != 0 {
		return status
	}

	if jsonOutput {
		return document(stdout, stderr, "debug-off", map[string]any{"xdebug": false})
	}

	fmt.Fprint(stdout, good("  xdebug disabled\n"))

	return exitOK
}

//
// Parsing php -v and php -i for debug-on's readback. Pure string -> value functions with no
// engine and no caller yet: debug-on itself lands in a later slice.
//

// xdebugVersion is the `grep "Xdebug v"` equivalent (debug-on.sh:27): it finds the line inside
// php -v's output that names Xdebug and its version, verbatim, and classifies it.
//
// version is that line, or "" when php -v carries no such line at all — the case the shell's own
// `set -euo pipefail` aborted on before ever reaching its own else clause (debug-on.sh:36).
// family is "v2" or "v3" when the line contains that substring, or "" for either an absent line
// or one that matches neither — the shell's own if/elif/else chain (debug-on.sh:31-36), ported
// directly rather than reinterpreted.
func xdebugVersion(phpVOutput string) (version, family string) {
	for _, line := range strings.Split(phpVOutput, "\n") {
		if strings.Contains(line, "Xdebug v") {
			version = line

			break
		}
	}

	switch {
	case strings.Contains(version, "v2."):
		family = "v2"
	case strings.Contains(version, "v3."):
		family = "v3"
	}

	return version, family
}

// xdebugSettingKeys is which ini keys each version family reads back — the exact sets
// debug-on.sh:32 and :34 grep for. v2 and v3 share xdebug.idekey and otherwise disagree: v2's
// settings are named remote_host/remote_port, v3's client_host/client_port. An unrecognized
// family, including "" for Xdebug absent, selects nothing.
func xdebugSettingKeys(family string) []string {
	switch family {
	case "v2":
		return []string{"xdebug.idekey", "xdebug.remote_host", "xdebug.remote_port"}
	case "v3":
		return []string{"xdebug.idekey", "xdebug.client_host", "xdebug.client_port"}
	default:
		return nil
	}
}

// xdebugSetting is one selected line of php -i's output, its ini key and the value read back for
// it — already split apart, rather than left as one "key=value" string, because a caller (the
// --json envelope) wants the value on its own.
type xdebugSetting struct {
	Key   string
	Value string
}

// xdebugSettings is the `php -i | grep -e ... -e ... -e ... | cut -d= -f1-2` equivalent
// (debug-on.sh:32, :34): for a recognized family it selects only the lines naming its own keys,
// in the order php -i itself printed them, and trims each to its key and value. A line naming a
// key outside the given family's set — the other family's host/port pair, on a fixture carrying
// both — is never selected, regardless of where it sits in the output.
func xdebugSettings(phpIOutput, family string) []xdebugSetting {
	keys := xdebugSettingKeys(family)
	if len(keys) == 0 {
		return nil
	}

	var selected []xdebugSetting

	for _, line := range strings.Split(phpIOutput, "\n") {
		key, ok := xdebugSettingKeyIn(line, keys)
		if !ok {
			continue
		}

		selected = append(selected, xdebugSetting{Key: key, Value: xdebugSettingValue(line)})
	}

	return selected
}

// xdebugSettingKeyIn reports which of the wanted keys, if any, this php -i line names.
func xdebugSettingKeyIn(line string, keys []string) (string, bool) {
	for _, key := range keys {
		if strings.Contains(line, key) {
			return key, true
		}
	}

	return "", false
}

// xdebugSettingValue trims a php -i line to its value — the `cut -d= -f1-2` equivalent's second
// half. php -i prints each setting's local value and its master value duplicated
// ("key => value => value"); this keeps only the first, with the stray leading ">" — php -i's
// own "key => value" column separator — and surrounding whitespace removed from both cuts.
func xdebugSettingValue(line string) string {
	_, after, found := strings.Cut(line, "=")
	if !found {
		return ""
	}

	value, _, _ := strings.Cut(after, "=")

	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), ">"))
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

	err := newEngine(stdout, stderr, jsonOutput).CopyInto(here(), args, all)
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

	if err := newEngine(stdout, stderr, jsonOutput).CopyFrom(here(), args); err != nil {
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
		if err := newEngine(stdout, stderr, jsonOutput).RemoveHost(domain); err != nil {
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

	if err := newEngine(stdout, stderr, jsonOutput).SetHost(here(), domain, database); err != nil {
		return report(stderr, jsonOutput, "set-host", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "set-host", map[string]any{
			"domain": domain, "database": database,
		})
	}

	return exitOK
}
