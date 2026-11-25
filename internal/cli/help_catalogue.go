package cli

import (
	"io"
	"sort"
	"strings"
)

//
// The capability catalogue: the help as one document, for a program that wants to know what the
// tool can do without parsing a page meant for people.
//
// `hm --json` and `hm help --json` are every command; `hm <command> --help --json` and
// `hm help <command> --json` are that command's entry. Everything in it is read from the embedded
// data/command_descriptions.json, the same declaration the page, the completion and the
// permissions are built from, so a command described there is described here and nothing has to
// be kept in step by hand. The one field the declaration does not hold, passthrough, is the
// router's own list (transparent, globals.go): the commands whose output, and whose flags after the
// name, belong to the program they run.
//

// catalogueOption is one option, as declared.
type catalogueOption struct {
	Short       string `json:"short"`
	Long        string `json:"long"`
	Description string `json:"description"`
}

// catalogueArgument is one argument, as declared.
type catalogueArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Multiple    bool   `json:"multiple"`
}

// catalogueEntry is one command. Every field is always there, empty when the declaration has
// nothing for it, so that a reader can rely on the shape.
type catalogueEntry struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Usage       string              `json:"usage"`
	Example     string              `json:"example"`
	Options     []catalogueOption   `json:"options"`
	Arguments   []catalogueArgument `json:"arguments"`
	Safety      string              `json:"safety"`
	Passthrough bool                `json:"passthrough"`
	Group       string              `json:"group"`
	MacOnly     bool                `json:"mac_only"`
}

type catalogueGroup struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// catalogueDocument is the whole catalogue: the commands by name, the groups in their declared
// order, and the options the tool accepts before any command.
type catalogueDocument struct {
	Commands      []catalogueEntry  `json:"commands"`
	Groups        []catalogueGroup  `json:"groups"`
	GlobalOptions []catalogueOption `json:"global_options"`
}

// helpTopic is the command `hm help <command>` asks about; nothing for `hm`, `hm --help` and
// `hm -h <anything>`, which have always been the whole list.
func helpTopic(rest []string) string {
	if len(rest) > 1 && rest[0] == "help" {
		return rest[1]
	}

	return ""
}

// helpInJSON decides the format of `hm <command> --help`. After a transparent command's name the
// global flags are left for the child (globals.go), so --json is still among the arguments there;
// with --help the child never runs, so it can only have been asked of the tool.
func helpInJSON(format string, after []string) bool {
	return format == "json" || (format == "" && contains(after, "--json"))
}

// catalogue answers the help in JSON. An empty name is every command.
func catalogue(name string, stdout, stderr io.Writer) int {
	descriptions, err := readAssetObject("data/command_descriptions.json")
	if err != nil {
		return failure(stderr, true, "help", exitError, "help_unavailable", err.Error(), "")
	}

	if name != "" {
		info, declared := descriptions[name].(map[string]any)
		if !declared || strings.HasPrefix(name, "_") {
			return failure(stderr, true, "help", exitUsage, "command_not_found",
				"Command not found: "+name, binaryName()+" --help")
		}

		return document(stdout, stderr, "help", catalogueEntryOf(name, info))
	}

	names := []string{}

	for key := range descriptions {
		if !strings.HasPrefix(key, "_") {
			names = append(names, key)
		}
	}

	sort.Strings(names)

	whole := catalogueDocument{
		Commands:      make([]catalogueEntry, 0, len(names)),
		Groups:        []catalogueGroup{},
		GlobalOptions: []catalogueOption{},
	}

	for _, command := range names {
		info, _ := descriptions[command].(map[string]any)
		whole.Commands = append(whole.Commands, catalogueEntryOf(command, info))
	}

	groups, _ := descriptions["_groups"].([]any)
	for _, entry := range groups {
		group, _ := entry.(map[string]any)
		whole.Groups = append(whole.Groups, catalogueGroup{ID: declaredString(group["id"]), Title: declaredString(group["title"])})
	}

	global, _ := descriptions["_global"].(map[string]any)
	whole.GlobalOptions = catalogueOptionsOf(global["opts"])

	return document(stdout, stderr, "help", whole)
}

// catalogueEntryOf is one command's declaration, as an entry. The group defaults to "other" and
// the usage to the bare name, as the page does for a command declared without them.
func catalogueEntryOf(name string, info map[string]any) catalogueEntry {
	usage, declared := usageOf(name, info)
	if !declared {
		usage = name
	}

	group := declaredString(info["group"])
	if group == "" {
		group = "other"
	}

	arguments := []catalogueArgument{}

	list, _ := info["args"].([]any)
	for _, entry := range list {
		argument, _ := entry.(map[string]any)
		multiple, _ := argument["multiple"].(bool)
		arguments = append(arguments, catalogueArgument{
			Name: declaredString(argument["name"]), Description: declaredString(argument["description"]), Multiple: multiple,
		})
	}

	macOnly, _ := info["mac"].(bool)

	return catalogueEntry{
		Name:        name,
		Description: declaredString(info["description"]),
		// The page's usage line ends in a space when there are options and no arguments, which is
		// the shell's printf and not part of the usage
		Usage:       strings.TrimRight(usage, " "),
		Example:     declaredString(info["example"]),
		Options:     catalogueOptionsOf(info["opts"]),
		Arguments:   arguments,
		Safety:      declaredString(info["safety"]),
		Passthrough: transparent[name],
		Group:       group,
		MacOnly:     macOnly,
	}
}

func catalogueOptionsOf(value any) []catalogueOption {
	options := []catalogueOption{}

	list, _ := value.([]any)
	for _, entry := range list {
		option, _ := entry.(map[string]any)
		names, _ := option["name"].(map[string]any)
		options = append(options, catalogueOption{
			Short: declaredString(names["short"]), Long: declaredString(names["long"]), Description: declaredString(option["description"]),
		})
	}

	return options
}

// declaredString is a declared string, or nothing when the declaration has none.
func declaredString(value any) string {
	typed, _ := value.(string)

	return typed
}
