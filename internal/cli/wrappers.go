package cli

import (
	"fmt"
	"io"
	"runtime"
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
	project, code := codeProjectOr(stderr, jsonOutput, "npm")
	if code != 0 {
		return code
	}

	return inside(project, append([]string{"npm"}, args...), "", stdout, stderr, jsonOutput, "npm")
}

// magerun is n98-magerun, which is in the image because half of what people ask Magento to do is
// quicker through it.
func magerun(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := codeProjectOr(stderr, jsonOutput, "n98-magerun")
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
	project, code := codeProjectOr(stderr, jsonOutput, "test-"+kind)
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
// debug-on, alone.
//
// It shares phpService, terminalOptions and copyFrom with the handlers above and needs no new
// engine surface beyond what Slice 2 already added: Exec, Restart, CopyFrom, Capture and
// XdebugHost are all already on commands.
//

func debugOn(stdout, stderr io.Writer, jsonOutput bool) int {
	project, code := projectOr(stderr, jsonOutput, "debug-on")
	if code != 0 {
		return code
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	// Consulted unconditionally, never the calling process's own environment: the shell's
	// undocumented $XDEBUG_HOST escape hatch is dropped on purpose (proposal Decision 8), so this
	// is the only thing that decides the address patched below. A plain error — an unresolved
	// docker0 on Linux — is a failure here, not a silent skip (unit 2's Decision 2, renumbered
	// Decision 4 in this proposal).
	host, err := engine.XdebugHost()
	if err != nil {
		return report(stderr, jsonOutput, "debug-on", err)
	}

	if host != "" {
		if code := debugOnPatchHost(engine, project.Root, "remote_host", host, stderr, jsonOutput); code != 0 {
			return code
		}

		if code := debugOnPatchHost(engine, project.Root, "client_host", host, stderr, jsonOutput); code != 0 {
			return code
		}
	}

	// Always runs, host or not — a different file from the two patches above. The shell's own
	// stray, inert `\;` before `zend_extension` is dropped, the same undefined-escape shape
	// debug-off's own `\z` was (proposal Decision 7).
	status, err := engine.Exec(project.Root, phpService,
		[]string{"sed", "-i", "-e", "s/^;zend_extension/zend_extension/g",
			"/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"},
		terminalOptions(""))
	if status != 0 {
		return failure(stderr, jsonOutput, "debug-on", status, "command_failed",
			"The xdebug extension uncomment edit failed in the phpfpm container", "")
	}
	if err != nil {
		return report(stderr, jsonOutput, "debug-on", err)
	}

	// Only on macOS the code lives in a volume rather than a mount, so the enabled extension's
	// generated files have to be copied back out. copyFrom prints nothing of its own on success
	// in text mode, so the line the shell script printed is written here instead (proposal
	// Decision 6); it is decorative, so — like every other line this handler writes before its
	// own document — it is skipped under --json. The same runtime.GOOS check php.go:100 already
	// uses — no new mechanism.
	//
	// copyFrom is always called in its own text mode here, regardless of debug-on's own
	// jsonOutput: unlike varnish's sibling calls (purge, magento), which never print a document
	// of their own, copyFrom does — under --json it would otherwise write its own
	// "copy-from-container" envelope onto stdout ahead of debug-on's, leaving two concatenated
	// top-level JSON values where a consumer expects exactly one. A failure here is rarer than
	// the edits' or the restart's own and has no covering spec scenario; it still reports through
	// copyFrom's own text-mode failure() rather than silently continuing.
	if runtime.GOOS == "darwin" {
		if !jsonOutput {
			fmt.Fprint(stdout, warning("Copying generated code into host\n"))
		}

		if code := copyFrom([]string{"generated"}, stdout, stderr, false); code != 0 {
			return code
		}
	}

	// Operator.Restart, not Compose — the opposite choice from debug-off's own, because
	// restart.sh is a faithful stop-then-start and Operator.Restart is what reproduces that,
	// worktree refusal included (proposal Decision 5, disclosed): hm debug-on has never refused
	// from an unregistered worktree before this port.
	if err := engine.Restart(project.Root, []string{phpService}); err != nil {
		return report(stderr, jsonOutput, "debug-on", err)
	}

	return debugOnReadback(engine, project.Root, host, stdout, stderr, jsonOutput)
}

// debugOnPatchHost is one of the two identical xdebug.ini edits, differing only in which key
// they patch — first remote_host (Xdebug v2), then client_host (v3), because which family the
// image ships is not known until the readback. The shell's stray, inert `\=` is dropped from
// both patterns, the same undefined-escape shape debug-off's own `\z` was (proposal Decision 7);
// `\.` is meaningful and stays.
func debugOnPatchHost(engine commands, root, key, host string, stderr io.Writer, jsonOutput bool) int {
	status, err := engine.Exec(root, phpService,
		[]string{"sed", "-i", `s/xdebug\.` + key + `=.*/xdebug\.` + key + `=` + host + `/g`,
			"/usr/local/etc/php/conf.d/xdebug.ini"},
		terminalOptions(""))
	if status != 0 {
		return failure(stderr, jsonOutput, "debug-on", status, "command_failed",
			"The xdebug.ini "+key+" edit failed in the phpfpm container", "")
	}
	if err != nil {
		return report(stderr, jsonOutput, "debug-on", err)
	}

	return 0
}

// xdebugSeparator is the shell's own frame around the readback (debug-on.sh:30, :38): forty-eight
// dashes, copied rather than retyped, because the two implementations are compared byte for byte
// elsewhere in this project and a miscounted rule is exactly the kind of difference that catches.
const xdebugSeparator = "------------------------------------------------"

// debugOnReadback runs the shell script's own final block (debug-on.sh:27-38): read the Xdebug
// version, classify it, and — for a recognized family — read the settings it selects. Both
// Capture calls are wrapped through failure() with a named step rather than returned as a raw
// status, unlike Restart's own passthrough above: a captured program's exit code is not something
// a caller of hm debug-on is meant to branch on the way a Compose subcommand's is.
func debugOnReadback(engine commands, root, host string, stdout, stderr io.Writer, jsonOutput bool) int {
	phpV, status, err := engine.Capture(root, phpService, []string{"php", "-v"})
	if err != nil {
		return failure(stderr, jsonOutput, "debug-on", exitDocker, "docker_failed",
			"Reading the Xdebug version failed: "+err.Error(), "")
	}
	if status != 0 {
		return failure(stderr, jsonOutput, "debug-on", status, "command_failed",
			"Reading the Xdebug version failed in the phpfpm container", "")
	}

	version, family := xdebugVersion(phpV)

	// Xdebug absent or unrecognized (Decision 1): the shell's own `set -euo pipefail` could
	// never reach its own else clause for the absent case; here it is reachable, and it is
	// chosen to be informative rather than fatal — the enabling work above already succeeded.
	if family == "" {
		if jsonOutput {
			return document(stdout, stderr, "debug-on", map[string]any{"xdebug": false})
		}

		printXdebugBanner(stdout, func() {
			fmt.Fprint(stdout, "Xdebug is not installed or the version is not recognized.\n")
		})

		return exitOK
	}

	phpI, status, err := engine.Capture(root, phpService, []string{"php", "-i"})
	if err != nil {
		return failure(stderr, jsonOutput, "debug-on", exitDocker, "docker_failed",
			"Reading its settings failed: "+err.Error(), "")
	}
	if status != 0 {
		return failure(stderr, jsonOutput, "debug-on", status, "command_failed",
			"Reading its settings failed in the phpfpm container", "")
	}

	settings := xdebugSettings(phpI, family)

	if jsonOutput {
		values := make(map[string]string, len(settings))
		for _, setting := range settings {
			values[setting.Key] = setting.Value
		}

		// A plain Go "" marshals to JSON "", not null — host is left as an untyped nil when
		// nothing was patched, which is what carries Decision 2's not-applicable/unresolved
		// distinction outward instead of flattening it into one falsy value.
		var reportedHost any
		if host != "" {
			reportedHost = host
		}

		return document(stdout, stderr, "debug-on", map[string]any{
			"xdebug": true, "version": version, "host": reportedHost, "settings": values,
		})
	}

	printXdebugBanner(stdout, func() {
		for _, setting := range settings {
			fmt.Fprintf(stdout, "%s => %s\n", setting.Key, setting.Value)
		}
	})

	return exitOK
}

// printXdebugBanner is the shell's own three-line frame around the readback: a header, a
// separator, whatever body prints, and the same separator again.
func printXdebugBanner(stdout io.Writer, body func()) {
	fmt.Fprint(stdout, warning("xdebug configuration:\n"))
	fmt.Fprint(stdout, warning(xdebugSeparator+"\n"))
	body()
	fmt.Fprint(stdout, warning(xdebugSeparator+"\n"))
}

//
// Parsing php -v and php -i for debug-on's readback. Pure string -> value functions, called from
// debugOnReadback above.
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
