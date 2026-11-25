package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// The capability catalogue: `hm --json`, `hm help --json` and `hm <command> --help --json`. Every
// field comes from data/command_descriptions.json, so the expectations here are read from that
// file too and never typed out — the catalogue must follow the file, not this test.
//

type catalogueOptionDoc struct {
	Short       string `json:"short"`
	Long        string `json:"long"`
	Description string `json:"description"`
}

type catalogueArgumentDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Multiple    bool   `json:"multiple"`
}

type catalogueEntryDoc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Usage       string                 `json:"usage"`
	Example     string                 `json:"example"`
	Options     []catalogueOptionDoc   `json:"options"`
	Arguments   []catalogueArgumentDoc `json:"arguments"`
	Safety      string                 `json:"safety"`
	Passthrough bool                   `json:"passthrough"`
	Group       string                 `json:"group"`
	MacOnly     bool                   `json:"mac_only"`
}

type catalogueDoc struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	OK            bool   `json:"ok"`
	Data          struct {
		Commands []catalogueEntryDoc `json:"commands"`
		Groups   []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"groups"`
		GlobalOptions []catalogueOptionDoc `json:"global_options"`
	} `json:"data"`
}

type catalogueEntryEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	Command       string            `json:"command"`
	OK            bool              `json:"ok"`
	Data          catalogueEntryDoc `json:"data"`
}

// declaredDescriptions is the tool's data/command_descriptions.json, as the test reads it.
func declaredDescriptions(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(helpTool, "data", "command_descriptions.json"))
	if err != nil {
		t.Fatal(err)
	}

	var descriptions map[string]json.RawMessage
	if err := json.Unmarshal(contents, &descriptions); err != nil {
		t.Fatal(err)
	}

	return descriptions
}

func catalogueIn(t *testing.T, args ...string) (catalogueDoc, string) {
	t.Helper()

	stdout, stderr, code := helpIn(t, t.TempDir(), false, args...)
	if code != exitOK || stderr != "" {
		t.Fatalf("hm %v: exit %d, stderr %q, want 0 and nothing", args, code, stderr)
	}

	var catalogue catalogueDoc
	if err := json.Unmarshal([]byte(stdout), &catalogue); err != nil {
		t.Fatalf("hm %v: stdout is not one JSON document: %v\n%s", args, err, stdout)
	}

	if catalogue.SchemaVersion != 1 || catalogue.Command != "help" || !catalogue.OK {
		t.Fatalf("hm %v: envelope = %d %q %v, want 1 \"help\" true", args,
			catalogue.SchemaVersion, catalogue.Command, catalogue.OK)
	}

	return catalogue, stdout
}

func TestTheCatalogueListsEveryDeclaredCommand(t *testing.T) {
	catalogue, _ := catalogueIn(t, "--json")

	want := []string{}
	for key := range declaredDescriptions(t) {
		if !strings.HasPrefix(key, "_") {
			want = append(want, key)
		}
	}

	sort.Strings(want)

	got := make([]string, 0, len(catalogue.Data.Commands))
	for _, entry := range catalogue.Data.Commands {
		got = append(got, entry.Name)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("commands (-want +got):\n%s", diff)
	}
}

// TestEveryEntryIsWhatTheFileDeclares compares each entry's fields with the declaration they come
// from: description, example, group, safety, options and arguments, field by field.
func TestEveryEntryIsWhatTheFileDeclares(t *testing.T) {
	catalogue, _ := catalogueIn(t, "--json")
	descriptions := declaredDescriptions(t)

	for _, entry := range catalogue.Data.Commands {
		var declared struct {
			Description string `json:"description"`
			Example     string `json:"example"`
			Group       string `json:"group"`
			Safety      string `json:"safety"`
			Mac         bool   `json:"mac"`
			Opts        []struct {
				Name struct {
					Short string `json:"short"`
					Long  string `json:"long"`
				} `json:"name"`
				Description string `json:"description"`
			} `json:"opts"`
			Args []catalogueArgumentDoc `json:"args"`
		}

		if err := json.Unmarshal(descriptions[entry.Name], &declared); err != nil {
			t.Fatalf("%s: %v", entry.Name, err)
		}

		options := []catalogueOptionDoc{}
		for _, option := range declared.Opts {
			options = append(options, catalogueOptionDoc{option.Name.Short, option.Name.Long, option.Description})
		}

		arguments := declared.Args
		if arguments == nil {
			arguments = []catalogueArgumentDoc{}
		}

		want := catalogueEntryDoc{
			Name: entry.Name, Description: declared.Description, Usage: entry.Usage,
			Example: declared.Example, Options: options, Arguments: arguments, Safety: declared.Safety,
			Passthrough: entry.Passthrough, Group: declared.Group, MacOnly: declared.Mac,
		}

		if diff := cmp.Diff(want, entry); diff != "" {
			t.Errorf("%s (-want +got):\n%s", entry.Name, diff)
		}

		if entry.Safety == "" || entry.Usage == "" {
			t.Errorf("%s: safety %q, usage %q, want both", entry.Name, entry.Safety, entry.Usage)
		}
	}
}

// TestTheUsageIsTheOneTheTextHelpPrints: declared when the file has one, built from the options
// and arguments when it does not, and the bare name when there is neither.
func TestTheUsageIsTheOneTheTextHelpPrints(t *testing.T) {
	catalogue, _ := catalogueIn(t, "--json")
	usages := map[string]string{}
	passthrough := map[string]bool{}
	macOnly := map[string]bool{}

	for _, entry := range catalogue.Data.Commands {
		usages[entry.Name] = entry.Usage
		passthrough[entry.Name] = entry.Passthrough
		macOnly[entry.Name] = entry.MacOnly
	}

	for name, usage := range map[string]string{
		"stop":  "stop [-t seconds] [--snapshot] [service...]",
		"start": "start [-s|--switch] <service-name>",
		// Declared, now that purge has an option (--dry-run)
		"purge": "purge [--dry-run]",
		// Nothing declared and nothing to take: the bare name
		"debug-off": "debug-off",
	} {
		if usages[name] != usage {
			t.Errorf("%s usage = %q, want %q", name, usages[name], usage)
		}
	}

	// The text help's own usage line for the same command, without the binary's name
	stdout, _, _ := helpIn(t, t.TempDir(), false, "start", "--help")
	if !strings.Contains(stdout, "Usage: hm "+usages["start"]) {
		t.Errorf("text help %q does not carry the usage %q", stdout, usages["start"])
	}

	if !passthrough["magento"] || !passthrough["exec"] || passthrough["start"] || passthrough["purge"] {
		t.Errorf("passthrough magento %v exec %v start %v purge %v, want true true false false",
			passthrough["magento"], passthrough["exec"], passthrough["start"], passthrough["purge"])
	}

	if !macOnly["copy-to-container"] || macOnly["start"] {
		t.Errorf("mac_only copy-to-container %v start %v, want true false",
			macOnly["copy-to-container"], macOnly["start"])
	}
}

func TestTheCatalogueCarriesTheGroupsAndTheGlobalOptions(t *testing.T) {
	catalogue, _ := catalogueIn(t, "--json")

	var declared struct {
		Groups []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"_groups"`
		Global struct {
			Opts []struct {
				Name struct {
					Short string `json:"short"`
					Long  string `json:"long"`
				} `json:"name"`
				Description string `json:"description"`
			} `json:"opts"`
		} `json:"_global"`
	}

	contents, err := os.ReadFile(filepath.Join(helpTool, "data", "command_descriptions.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(contents, &declared); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(declared.Groups, catalogue.Data.Groups); diff != "" {
		t.Errorf("groups (-want +got):\n%s", diff)
	}

	options := []catalogueOptionDoc{}
	for _, option := range declared.Global.Opts {
		options = append(options, catalogueOptionDoc{option.Name.Short, option.Name.Long, option.Description})
	}

	if diff := cmp.Diff(options, catalogue.Data.GlobalOptions); diff != "" {
		t.Errorf("global options (-want +got):\n%s", diff)
	}
}

// TestEverySpellingOfTheCatalogueIsTheSameDocument: `hm --json`, `hm help --json`, `hm --help
// --json` and `hm -h --json` print the same bytes.
func TestEverySpellingOfTheCatalogueIsTheSameDocument(t *testing.T) {
	_, want := catalogueIn(t, "--json")

	for _, args := range [][]string{{"help", "--json"}, {"--help", "--json"}, {"--json", "-h"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, got := catalogueIn(t, args...); got != want {
				t.Fatalf("hm %v differs from hm --json", args)
			}
		})
	}
}

// TestACommandsHelpInJSONIsItsEntry: `hm <command> --help --json` and `hm help <command> --json`
// are that command's entry in the catalogue, whole.
func TestACommandsHelpInJSONIsItsEntry(t *testing.T) {
	catalogue, _ := catalogueIn(t, "--json")

	entries := map[string]catalogueEntryDoc{}
	for _, entry := range catalogue.Data.Commands {
		entries[entry.Name] = entry
	}

	for _, args := range [][]string{
		{"start", "--help", "--json"}, {"--json", "db", "-h"}, {"help", "magento", "--json"},
		// After a transparent command's name --json is left for the child, which --help never runs
		{"magento", "--help", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := helpIn(t, t.TempDir(), false, args...)
			if code != exitOK || stderr != "" {
				t.Fatalf("exit %d, stderr %q, want 0 and nothing", code, stderr)
			}

			var envelope catalogueEntryEnvelope
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
			}

			if envelope.SchemaVersion != 1 || envelope.Command != "help" || !envelope.OK {
				t.Fatalf("envelope = %d %q %v, want 1 \"help\" true",
					envelope.SchemaVersion, envelope.Command, envelope.OK)
			}

			name := args[0]
			if name == "--json" || name == "help" {
				name = args[1]
			}

			if diff := cmp.Diff(entries[name], envelope.Data); diff != "" {
				t.Fatalf("entry (-want +got):\n%s", diff)
			}
		})
	}
}

// TestAnUndeclaredCommandsHelpInJSONIsAFailure: the text help prints an empty line for a command
// described nowhere; a JSON reader gets the error envelope command_not_found, on stderr.
func TestAnUndeclaredCommandsHelpInJSONIsAFailure(t *testing.T) {
	stdout, stderr, code := helpIn(t, t.TempDir(), false, "no-such-command", "--help", "--json")
	if code != exitUsage || stdout != "" {
		t.Fatalf("exit %d, stdout %q, want exitUsage and nothing", code, stdout)
	}

	var envelope struct {
		Command string `json:"command"`
		OK      bool   `json:"ok"`
		Error   struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr)
	}

	if envelope.Command != "help" || envelope.OK || envelope.Error.Code != exitUsage ||
		envelope.Error.Type != "command_not_found" || envelope.Error.Message != "Command not found: no-such-command" {
		t.Fatalf("envelope = %+v", envelope)
	}
}

// TestHelpWithoutJSONIsStillNotACommand: `hm help` is the catalogue only when JSON is asked for;
// in text it is what it was, a name the tool does not know.
func TestHelpWithoutJSONIsStillNotACommand(t *testing.T) {
	stdout, _, code := helpIn(t, t.TempDir(), false, "help", "--no-json")
	if code != exitUsage || stdout != "" {
		t.Fatalf("hm help --no-json: exit %d, stdout %q, want exitUsage and nothing", code, stdout)
	}
}
