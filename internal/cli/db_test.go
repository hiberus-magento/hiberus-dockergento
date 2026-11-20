package cli

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What console/commands/db.sh answered without reaching either family: the usage text when it
// was given no subcommand, and the refusal of one it did not know. Neither needs a project nor
// Docker — answering(t) stands in a temporary directory that is not one, with no daemon — and
// neither touches the engine.
//

const dbUsageHeadings = "Named copies of this project's database\n\n" +
	"Frozen copies of the data directory, to stand environments up in seconds\n\n"

const dbUsageLines = "  hm db snapshot [--name=<name>] [--force]\n" +
	"  hm db list\n" +
	"  hm db restore <name>\n" +
	"  hm db remove <name>\n" +
	"  hm db clear [--all]\n\n" +
	"  hm db freeze [--name=<name>] [--force]\n" +
	"  hm db templates\n" +
	"  hm db clone [<project>/]<name> [--force]\n" +
	"  hm db drop [<project>/]<name>\n\n"

func TestDbWithoutASubcommandPrintsTheUsage(t *testing.T) {
	for _, args := range [][]string{{"--no-json", "db"}, {"--no-json", "db", ""}} {
		fake := answering(t)
		t.Setenv("NO_COLOR", "1")

		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != exitOK {
			t.Fatalf("%q = %d, want exitOK\nstderr: %s", args, code, stderr.String())
		}

		want := "Named copies of this project's database\n\n" +
			"  hm db snapshot [--name=<name>] [--force]\n" +
			"  hm db list\n" +
			"  hm db restore <name>\n" +
			"  hm db remove <name>\n" +
			"  hm db clear [--all]\n\n" +
			"Frozen copies of the data directory, to stand environments up in seconds\n\n" +
			"  hm db freeze [--name=<name>] [--force]\n" +
			"  hm db templates\n" +
			"  hm db clone [<project>/]<name> [--force]\n" +
			"  hm db drop [<project>/]<name>\n\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Errorf("%q stdout (-want +got):\n%s", args, diff)
		}

		if stderr.Len() != 0 {
			t.Errorf("%q stderr = %q, want nothing", args, stderr.String())
		}

		if len(fake.calls) != 0 {
			t.Errorf("%q calls = %v, want none", args, fake.calls)
		}
	}
}

// Under --json the script's print_info went to stderr and its print_default stayed on stdout
// (console/components/print_message.sh), so the usage arrives split in two, and still exit 0.
func TestDbUsageUnderJSONKeepsTheScriptsStreams(t *testing.T) {
	fake := answering(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "db"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("db = %d, want exitOK\nstderr: %s", code, stderr.String())
	}

	if diff := cmp.Diff(dbUsageLines, stdout.String()); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(dbUsageHeadings, stderr.String()); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}

	if len(fake.calls) != 0 {
		t.Errorf("calls = %v, want none", fake.calls)
	}
}

func TestDbRefusesAnUnknownSubcommand(t *testing.T) {
	fake := answering(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "db", "bogus"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("db bogus = %d, want exitUsage\nstderr: %s", code, stderr.String())
	}

	want := `{
  "schema_version": 1,
  "command": "db",
  "ok": false,
  "error": {
    "code": 2,
    "type": "unknown_subcommand",
    "message": "'bogus' is not something hm db does",
    "hint": "hm db snapshot | list | restore | remove | clear | freeze | templates | clone | drop"
  }
}
`
	if diff := cmp.Diff(want, stderr.String()); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}

	if len(fake.calls) != 0 {
		t.Errorf("calls = %v, want none", fake.calls)
	}
}

// An option where the subcommand goes is a subcommand nobody knows, as it was for the script
func TestDbRefusesAnOptionInPlaceOfTheSubcommand(t *testing.T) {
	answering(t)

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "db", "--name=x"}, &bytes.Buffer{}, &stderr); code != exitUsage {
		t.Fatalf("db --name=x = %d, want exitUsage\nstderr: %s", code, stderr.String())
	}

	if want := `"message": "'--name=x' is not something hm db does"`; !bytes.Contains(stderr.Bytes(), []byte(want)) {
		t.Errorf("stderr = %s, want it to contain %s", stderr.String(), want)
	}
}

// print_default opened every line with a colour reset and print_info painted the headings green,
// so under colour the usage carries the same escapes it did
func TestDbUsageInColourKeepsTheScriptsEscapes(t *testing.T) {
	answering(t)
	t.Setenv("NO_COLOR", "")
	t.Setenv("HM_NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	t.Setenv("FORCE_COLOR", "1")

	var stdout bytes.Buffer
	if code := Run([]string{"--no-json", "db"}, &stdout, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("db = %d, want exitOK", code)
	}

	const green, reset = "\033[0;32m", "\033[0m"

	want := green + "Named copies of this project's database\n\n" + reset +
		reset + "  hm db snapshot [--name=<name>] [--force]\n"
	if got := stdout.String(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("stdout starts %q, want it to start %q", got, want)
	}
}
