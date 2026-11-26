package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

//
// When error 1412 keeps coming back after every retry, the table is almost always one a live
// store keeps altering, and dumping it without data is the fix. The failure names it and gives
// the exact line for config/docker/properties.json: a suggestion, never applied by itself.
//

// customTableChanged is error 1412 on a table no default rule covers: a project's own.
const customTableChanged = "mariadb-dump: Error 1412: Table definition has changed, please retry transaction when dumping table `vendor_import_queue` at row: 0\n" +
	"hm-transfer-db: the dump exited with status 2\n"

// hintIn is the hint of the --json failure.
func hintIn(t *testing.T, stderr []byte) string {
	t.Helper()

	var failed struct {
		Error struct {
			Hint string `json:"hint"`
		} `json:"error"`
	}

	document := stderr
	if at := bytes.IndexByte(document, '{'); at >= 0 {
		document = document[at:]
	}

	if err := json.Unmarshal(document, &failed); err != nil {
		t.Fatalf("stderr is not a failure document: %v\n%s", err, stderr)
	}

	return failed.Error.Hint
}

// TestAnExhausted1412SuggestsTheLine: the table is named, with the line that would dump it
// without data — the project's list kept, the table added.
func TestAnExhausted1412SuggestsTheLine(t *testing.T) {
	cases := []struct {
		name, property string
		args           []string
		hint           string
	}{
		{
			name: "after the project's list", property: "@saas,my_log",
			hint: "To dump `vendor_import_queue` without its data from now on, set this in config/docker/properties.json:\n" +
				`"TRANSFER_DB_STRUCTURE_ONLY": "@saas,my_log,vendor_import_queue"`,
		},
		{
			name: "alone when the project's list is empty", property: "",
			hint: "To dump `vendor_import_queue` without its data from now on, set this in config/docker/properties.json:\n" +
				`"TRANSFER_DB_STRUCTURE_ONLY": "vendor_import_queue"`,
		},
		{
			name: "the run's rules are not the project's", property: "@saas", args: []string{"--structure-only=tmp_*"},
			hint: "To dump `vendor_import_queue` without its data from now on, set this in config/docker/properties.json:\n" +
				`"TRANSFER_DB_STRUCTURE_ONLY": "@saas,vendor_import_queue"`,
		},
		{
			name: "a table the rules already name, under --full", property: "@saas,vendor_*", args: []string{"--full"},
			hint: "`vendor_import_queue` is in TRANSFER_DB_STRUCTURE_ONLY already: run without --full to dump it without its data",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake, ran := dbLab(t)
			fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": tt.property}
			sshAnswering(t, ran, sshOutcome{said: customTableChanged})

			var stderr bytes.Buffer

			if code := transferDB(append([]string{"--ssh-user=abc"}, tt.args...), io.Discard, &stderr, true); code != 2 {
				t.Fatalf("transfer-db = %d, want the dump's 2", code)
			}

			if len(*ran) != 3 {
				t.Fatalf("ssh ran %d times, want three attempts", len(*ran))
			}

			if got := hintIn(t, stderr.Bytes()); got != tt.hint {
				t.Fatalf("hint =\n%s\nwant\n%s", got, tt.hint)
			}

			_, message := failureIn(t, stderr.Bytes())
			mustContain(t, "the failure", message, "failed after 3 attempts", "`vendor_import_queue`", "The whole transfer is logged in")
		})
	}
}

// TestTheSuggestionIsPrintedUnderTheFailure on a terminal, where it can be copied as it is.
func TestTheSuggestionIsPrintedUnderTheFailure(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}
	sshAnswering(t, ran, sshOutcome{said: customTableChanged})

	var stderr bytes.Buffer

	transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, false)

	mustContain(t, "stderr", stderr.String(), "The whole transfer is logged in "+transferLogPath(t)+"\n",
		"  → To dump `vendor_import_queue` without its data from now on, set this in config/docker/properties.json:\n"+
			`"TRANSFER_DB_STRUCTURE_ONLY": "@saas,vendor_import_queue"`+"\n")
}

// TestNoSuggestionWithoutATableToName: a 1412 the rules already cover, one that did not say its
// table, and every other failure end as they did.
func TestNoSuggestionWithoutATableToName(t *testing.T) {
	cases := []struct {
		name, property, said string
	}{
		{name: "a table the rules already name", property: "@saas,vendor_*", said: customTableChanged},
		{
			name: "no table in the message", property: "@saas",
			said: "mariadb-dump: Error 1412: Table definition has changed, please retry transaction\nhm-transfer-db: the dump exited with status 2\n",
		},
		{name: "another error", property: "@saas", said: lostConnection + "hm-transfer-db: the dump exited with status 2\n"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake, ran := dbLab(t)
			fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": tt.property}
			sshAnswering(t, ran, sshOutcome{said: tt.said})

			var stderr bytes.Buffer

			if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, true); code != 2 {
				t.Fatalf("transfer-db = %d, want the dump's 2", code)
			}

			if got := hintIn(t, stderr.Bytes()); got != "" {
				t.Fatalf("hint = %q, want none", got)
			}
		})
	}
}
