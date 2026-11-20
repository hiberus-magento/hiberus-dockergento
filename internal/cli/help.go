package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
)

//
// `hm`, `hm --help` and `hm <command> --help`.
//
// Ported from console/helpers/print_help.sh and print_usage.sh, and compared against what they
// print by the goldens in testdata/help, which the real bin/run produced. That is why so much of
// this reads like jq and printf rather than like Go: a value is printed the way `jq -r` prints it,
// text through a print_* helper has its escapes expanded the way `printf %b` expands them, and a
// width is counted in bytes. A difference of one escape sequence is a difference.
//
// The help has no JSON form, in either implementation: it is the output of the command, so all of
// it goes to stdout whatever the format, and its colour follows the palette's one decision.
//
// Two things are deliberately not carried over. The shell listed any `console/commands/*.sh` it
// found as a command of the tool; that directory is gone, and a script dropped there now would be
// listed for a command nothing runs. And names are ordered by byte, which is what the shell's
// `sort(1)` and glob did on macOS and under the C locale; under a Linux UTF-8 locale they collated
// instead, so `dbeaver` came before `db` there.
//

// helpMachine is the platform the mac-only commands are kept or dropped for: a test pins it, since
// the goldens were captured on macOS.
var helpMachine = dockergento.Platform

// helpTerminal is whether somebody is watching stdout, which is what decides the logo.
var helpTerminal = func(stdout io.Writer) bool { return isTerminal(stdout) }

const (
	helpGreen  = "\033[0;32m"
	helpBrown  = "\033[0;33m"
	helpPurple = "\033[0;35m"
	helpBlue   = "\033[0;34m"
	helpBold   = "\033[1m"
	helpReset  = "\033[0m"
)

// helpPage writes one page. It is built whole and written once.
type helpPage struct {
	out    strings.Builder
	colour bool
}

func (p *helpPage) code(sequence string) string {
	if !p.colour {
		return ""
	}

	return sequence
}

//
// Every text is written as it is. The shell passed these through printf's %b, and pasted a
// command's description into the format itself, so a `%`, a backslash or a `\c` in
// command_descriptions.json was interpreted; with bin/run gone there is nothing to be
// byte-identical with for such data, and text from a data file is printed literally.
//

// decorated is _print_decorated in text mode: print_info, print_heading.
func (p *helpPage) decorated(colour, text string) {
	p.out.WriteString(p.code(colour) + text + p.code(helpReset))
}

// codeText is print_code.
func (p *helpPage) codeText(text string) {
	p.out.WriteString(p.code(helpBrown) + text + p.code(helpReset))
}

// plain is print_default, which resets before it prints rather than after.
func (p *helpPage) plain(text string) {
	p.out.WriteString(p.code(helpReset) + text)
}

// helpData is what the help is built from: the tool's command_descriptions.json, and the project's
// own commands with their descriptions when it has any.
type helpData struct {
	descriptions map[string]any
	customDir    string
}

func loadHelpData(root string) (helpData, error) {
	descriptions, err := readAssetObject("data/command_descriptions.json")
	if err != nil {
		return helpData{}, err
	}

	return helpData{descriptions: descriptions, customDir: customCommandsDir(root)}, nil
}

func readJSONObject(path string) (map[string]any, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var object map[string]any
	if err := json.Unmarshal(contents, &object); err != nil {
		return nil, err
	}

	return object, nil
}

// help answers `hm`, `hm --help` and `hm <command> --help`. An empty name is the whole list.
func help(name string, stdout, stderr io.Writer) int {
	data, err := loadHelpData(resolveHMRoot(here()).Root)
	if err != nil {
		return failure(stderr, false, "help", exitError, "help_unavailable", err.Error(), "")
	}

	page := &helpPage{colour: coloured()}

	if name == "" {
		data.everything(page, helpTerminal(stdout))
	} else {
		data.command(page, name)
	}

	io.WriteString(stdout, page.out.String()) //nolint:errcheck

	return exitOK
}

// everything is print_all_commands_help_info.
func (d helpData) everything(p *helpPage, terminal bool) {
	if terminal {
		p.out.WriteString(logo(p))
	}

	p.decorated(helpBold, "Usage: ")
	p.plain(binaryName() + " <command> [options]\n")

	d.commands(p, d.descriptions, d.toolNames(), false)
	d.customCommands(p)
	d.examples(p)
	d.globalOptions(p)

	p.plain("Run ")
	p.codeText(binaryName() + " <command> --help")
	p.plain(" for details on a single command.\n\n")
}

// logo is print_logo in console/tasks/copyright.sh: block characters on a UTF-8 locale, ASCII
// otherwise, and only for a terminal.
func logo(p *helpPage) string {
	charset := os.Getenv("LC_ALL")
	if charset == "" {
		charset = os.Getenv("LC_CTYPE")
	}

	if charset == "" {
		charset = os.Getenv("LANG")
	}

	drawing := "H   H  M   M   Hiberus Dockergento\n" +
		"HHHHH  M M M   Docker environments for Magento 2\n" +
		"H   H  M   M\n"

	for _, marker := range []string{"UTF-8", "utf-8", "UTF8", "utf8"} {
		if strings.Contains(charset, marker) {
			drawing = "█  █ █▀▄▀█   Hiberus Dockergento\n" +
				"█▀▀█ █ ▀ █   Docker environments for Magento 2\n" +
				"▀  ▀ ▀   ▀\n"

			break
		}
	}

	return p.code(helpBlue) + drawing + p.code(helpReset) + "\n"
}

// toolNames is every declared command, as the shell's sort ordered them: with the `.sh` they had
// as file names, by byte.
func (d helpData) toolNames() []string {
	names := []string{}

	for key := range d.descriptions {
		if !strings.HasPrefix(key, "_") {
			names = append(names, key+".sh")
		}
	}

	sort.Strings(names)

	return names
}

// customCommands is the project's own pass: every script in its commands directory, described by
// the descriptions file beside them or, without one, by the tool's.
func (d helpData) customCommands(p *helpPage) {
	if info, err := os.Stat(d.customDir); err != nil || !info.IsDir() {
		return
	}

	// The directory as read, not a glob of its path: a project path holding `[`, `*` or `?` would
	// make the pattern match nothing. A name starting with `.` is skipped, as bash's `*` skipped it.
	entries, _ := os.ReadDir(d.customDir)
	names := []string{}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".sh") {
			continue
		}

		if info, err := os.Stat(filepath.Join(d.customDir, name)); err == nil && info.Mode().IsRegular() {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	descriptions := d.descriptions

	if _, err := os.Stat(filepath.Join(d.customDir, "command_descriptions.json")); err == nil {
		own, err := readJSONObject(filepath.Join(d.customDir, "command_descriptions.json"))
		if err != nil {
			// A broken descriptions file skips only this section, the project's own commands: the
			// tool's commands, the examples and the global options are still printed
			return
		}

		descriptions = own
	}

	d.commands(p, descriptions, names, true)
}

type helpRow struct {
	group, title, name, description string
	macOnly                         bool
	rank                            int
}

// commands is print_commands_info: one table, grouped in the declared order of the groups and, in
// a group, in the order of the names.
func (d helpData) commands(p *helpPage, descriptions map[string]any, names []string, custom bool) {
	if len(names) == 0 {
		return
	}

	order := map[string]int{}
	titles := map[string]any{}

	groups, _ := descriptions["_groups"].([]any)
	for position, entry := range groups {
		group, _ := entry.(map[string]any)
		id := jqRaw(group["id"])

		if _, seen := order[id]; !seen {
			order[id] = position
		}

		titles[id] = group["title"]
	}

	rows := make([]helpRow, 0, len(names))

	for _, file := range names {
		name := strings.TrimSuffix(file, ".sh")
		entry, _ := descriptions[name].(map[string]any)

		row := helpRow{
			name:        name,
			description: jqRaw(alternative(entry["description"], "")),
			macOnly:     jqRaw(alternative(entry["mac"], false)) == "true",
			group:       jqRaw(alternative(entry["group"], "other")),
		}

		row.title = jqRaw(alternative(titles[row.group], "Other"))

		row.rank = 999
		if position, found := order[row.group]; found {
			row.rank = position
		}

		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].rank < rows[j].rank })

	colour := helpGreen
	if custom {
		colour = helpPurple
	}

	current := ""

	for _, row := range rows {
		if row.name == "" || (helpMachine() != "mac" && row.macOnly) {
			continue
		}

		if row.group != current {
			current = row.group

			title := row.title
			if custom {
				title = "Custom commands"
			}

			p.out.WriteString("\n")
			p.decorated(helpBold, title+"\n")
		}

		p.out.WriteString("  " + p.code(colour) + pad(row.name, 22, true) + p.code(helpReset) + row.description + "\n")
	}
}

// examples is print_examples.
func (d helpData) examples(p *helpPage) {
	entries, _ := d.descriptions["_examples"].([]any)
	if len(entries) == 0 {
		return
	}

	p.out.WriteString("\n")
	p.decorated(helpBold, "Examples\n")

	for _, entry := range entries {
		example, _ := entry.(map[string]any)
		command := jqRaw(alternative(example["command"], ""))

		if command == "" {
			continue
		}

		p.out.WriteString("  " + p.code(helpGreen) + pad(command, 30, true) + p.code(helpReset) +
			jqRaw(alternative(example["description"], "")) + "\n")
	}
}

// globalOptions is print_global_options.
func (d helpData) globalOptions(p *helpPage) {
	global, _ := d.descriptions["_global"].(map[string]any)

	options, _ := global["opts"].([]any)
	if len(options) == 0 {
		return
	}

	p.out.WriteString("\n")
	p.decorated(helpBold, "Global options\n")

	for _, entry := range options {
		option, _ := entry.(map[string]any)
		name, _ := option["name"].(map[string]any)
		long := jqRaw(alternative(name["long"], ""))

		if long == "" {
			continue
		}

		p.out.WriteString("  " + p.code(helpBrown) + pad("--"+long, 22, true) + p.code(helpReset) +
			jqRaw(alternative(option["description"], "")) + "\n")
	}

	p.out.WriteString("\n")
}

// command is `usage <name>` in print_help.sh: the tool's description of a command or, when the
// tool has none, the project's.
//
// The sections are decided the way the shell decided them, by looking for the section's name in
// the command's keys run together — so a key that merely contains "example" asks for the example.
// A command described nowhere prints one empty line and succeeds, as it always has.
func (d helpData) command(p *helpPage, name string) {
	entry, found := d.descriptions[name]

	if !found || entry == nil {
		if own, err := readJSONObject(filepath.Join(d.customDir, "command_descriptions.json")); err == nil {
			entry = own[name]
		}
	}

	info, _ := entry.(map[string]any)
	keys := make([]string, 0, len(info))

	for key := range info {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	params := "false"
	if len(keys) > 0 {
		params = strings.Join(keys, "\n")
	}

	if usage := chomp(jqRaw(info["usage"])); usage != "null" {
		p.decorated(helpGreen, "Usage: ")
		p.codeText(binaryName() + " " + usage + "\n")
	} else if strings.Contains(params, "args") || strings.Contains(params, "opts") {
		p.decorated(helpGreen, "Usage: ")
		p.codeText(binaryName() + " " + name + " " + usageOptions(info["opts"]) + usageArguments(info["args"]) + "\n")
	}

	if strings.Contains(params, "example") {
		p.decorated(helpGreen, "Example: ")
		p.codeText(binaryName() + " " + chomp(jqRaw(info["example"])) + "\n")
	}

	if strings.Contains(params, "description") {
		p.decorated(helpGreen, "Description:")
		// The shell's `%1s` with no argument: one space
		p.out.WriteString(" " + chomp(jqRaw(info["description"])) + "\n")
	}

	if strings.Contains(params, "opts") {
		d.entries(p, info["opts"], "Options:\n", func(item map[string]any) (string, string) {
			option, _ := item["name"].(map[string]any)

			return "[" + jqAdd(jqAdd(jqAdd("-", option["short"]), "|--"), option["long"]) + "]",
				" " + chomp(jqRaw(item["description"]))
		})
	}

	if strings.Contains(params, "args") {
		d.entries(p, info["args"], "Arguments:\n", func(item map[string]any) (string, string) {
			return "<" + chomp(jqRaw(item["name"])) + ">", chomp(jqRaw(item["description"]))
		})
	}

	p.out.WriteString("\n")
}

// entries is print_opts and print_args: a heading when there is anything, then one line each.
func (d helpData) entries(p *helpPage, value any, heading string, line func(map[string]any) (string, string)) {
	list, _ := value.([]any)

	if len(list) > 0 {
		p.decorated(helpGreen, heading)
	}

	for _, entry := range list {
		item, _ := entry.(map[string]any)
		name, description := line(item)
		p.out.WriteString("   " + p.code(helpBrown) + pad(name, 16, true) + p.code(helpReset) + description + "\n")
	}
}

// usageOptions is the options half of a usage line built from the declaration.
func usageOptions(value any) string {
	if !jqTruthy(value) {
		return ""
	}

	list, _ := value.([]any)
	parts := make([]string, 0, len(list))

	for _, entry := range list {
		item, _ := entry.(map[string]any)
		option, _ := item["name"].(map[string]any)
		parts = append(parts, jqAdd(jqAdd(jqAdd(jqAdd("[-", option["short"]), "|--"), option["long"]), "]"))
	}

	return strings.Join(parts, " ") + " "
}

// usageArguments is the arguments half.
func usageArguments(value any) string {
	if !jqTruthy(value) {
		return ""
	}

	list, _ := value.([]any)
	parts := make([]string, 0, len(list))

	for _, entry := range list {
		item, _ := entry.(map[string]any)

		if jqTruthy(item["multiple"]) {
			parts = append(parts, jqAdd(jqAdd(jqAdd(jqAdd("<", item["name"]), "1>...<"), item["name"]), "N>"))
		} else {
			parts = append(parts, jqAdd(jqAdd("<", item["name"]), ">"))
		}
	}

	return strings.Join(parts, " ")
}

//
// jq, as far as the help needs it.
//

// jqRaw is a value as `jq -r` prints it.
func jqRaw(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}

	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}

	return string(pretty)
}

// alternative is jq's `//`: the fallback when the value is null or false.
func alternative(value, fallback any) any {
	if !jqTruthy(value) {
		return fallback
	}

	return value
}

func jqTruthy(value any) bool {
	return value != nil && value != false
}

// jqAdd is a string plus a value, where null adds nothing.
func jqAdd(text string, value any) string {
	if value == nil {
		return text
	}

	return text + jqRaw(value)
}

// chomp is what a command substitution does to a value: trailing newlines go.
func chomp(text string) string {
	return strings.TrimRight(text, "\n")
}

// pad is printf's width: counted in bytes, as bash counts it.
func pad(text string, width int, left bool) string {
	if len(text) >= width {
		return text
	}

	fill := strings.Repeat(" ", width-len(text))
	if left {
		return text + fill
	}

	return fill + text
}
