package cli

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Structure-only tables: the ones a dump takes without their data, because nobody needs their
// rows on a developer's machine and because a live store alters them while it is being dumped —
// the data exporter's batch tables are what made a user's transfer fail with error 1412. Built-in
// groups named as n98-magerun2 names them, a project's own TRANSFER_DB_STRUCTURE_ONLY, and
// --structure-only and --full for one run. mariadb-dump's --ignore-table has no wildcards, so the
// patterns are resolved on the server the dump is taken from, in the same SSH session.
//

// structureOnlyGolden is the script a run with rules sends: kept in testdata, where a reviewer
// reads it as the shell it is. Absolute, resolved when the package loads: dbLab moves the test
// into a project directory of its own.
var structureOnlyGolden, _ = filepath.Abs("testdata/transfer-db/structure-only-dump.golden")

func structureOnlyScript(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile(structureOnlyGolden)
	if err != nil {
		t.Fatal(err)
	}

	return strings.TrimSuffix(string(contents), "\n")
}

// structureOnlyLines is what the script prints on stderr for each table it dumps without data.
func structureOnlyLines(tables ...string) string {
	said := ""
	for _, table := range tables {
		said += "hm-transfer-db-structure-only: " + table + "\n"
	}

	return said
}

// TestTheDefaultStructureOnlyRuleIsTheSaasGroup: a project that says nothing dumps the data
// exporter's feeds and batches without data, and every n98-magerun2 group is there by its name.
func TestTheDefaultStructureOnlyRuleIsTheSaasGroup(t *testing.T) {
	properties, err := readAssetObject("data/properties.json")
	if err != nil {
		t.Fatal(err)
	}

	if got := properties["TRANSFER_DB_STRUCTURE_ONLY"]; got != "@saas" {
		t.Fatalf("default TRANSFER_DB_STRUCTURE_ONLY = %v, want @saas", got)
	}

	contents, err := fs.ReadFile(toolAssets(), "data/table-groups.json")
	if err != nil {
		t.Fatal(err)
	}

	groups, err := core.ParseTableGroups(contents)
	if err != nil {
		t.Fatal(err)
	}

	ids := []string{}
	for _, group := range groups {
		ids = append(ids, group.ID)
	}

	for _, id := range []string{"admin", "oauth", "log", "sessions", "stripped", "sales", "quotes", "customers", "trade",
		"development", "ee_changelog", "indexer_batches", "search", "idx", "dotmailer", "2fa", "newrelic_reporting",
		"aggregated", "replica", "klarna", "temp", "mailchimp", "saas"} {
		if !contains(ids, id) {
			t.Errorf("table group @%s is missing: %v", id, ids)
		}
	}

	// Every group resolves: no unknown group inside another, no pattern the script cannot carry
	for _, id := range ids {
		if _, err := core.StructureOnlyPatterns(groups, []string{"@" + id}); err != nil {
			t.Errorf("@%s does not resolve: %v", id, err)
		}
	}

	patterns, _ := core.StructureOnlyPatterns(groups, []string{"@saas"})
	for _, table := range []string{"sales_data_exporter_orders_v2_sync_batches", "cde_products_feed", "cde_product_prices_feed",
		"catalog_data_exporter_products", "sales_data_exporter_orders_v2", "cde_products_feed_index_batches"} {
		if !matchesAny(patterns, table) {
			t.Errorf("@saas does not take %s without data: %v", table, patterns)
		}
	}

	for _, table := range []string{"sales_order", "catalog_product_entity", "data_exporter_uuid", "core_config_data"} {
		if matchesAny(patterns, table) {
			t.Errorf("@saas takes %s without data: %v", table, patterns)
		}
	}
}

func matchesAny(patterns []string, table string) bool {
	for _, pattern := range patterns {
		if core.TableMatches(pattern, table) {
			return true
		}
	}

	return false
}

// TestStructureOnlyRulesDumpTheMatchedTablesWithoutData: the project's rules and the run's add
// up, the script resolves them on the server, and the run says which tables came without data.
func TestStructureOnlyRulesDumpTheMatchedTablesWithoutData(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas,my_log"}
	sshAnswering(t, ran, sshOutcome{said: structureOnlyLines("cde_products_feed", "my_log")})

	var stdout bytes.Buffer

	code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw", "--structure-only=tmp_*", "--anonymise=false"}, &stdout, io.Discard, false)
	if code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	// UPDATE_GOLDEN=1 rewrites the script's golden from what was sent: read the diff before keeping it
	if os.Getenv("UPDATE_GOLDEN") != "" && len(*ran) == 1 {
		if err := os.WriteFile(structureOnlyGolden, []byte((*ran)[0].Command()+"\n"), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	if len(*ran) != 1 || (*ran)[0].Command() != structureOnlyScript(t) {
		t.Fatalf("ssh ran\n%s\nwant\n%s", (*ran)[0].Command(), structureOnlyScript(t))
	}

	mustContain(t, "stdout", stdout.String(),
		"Tables to dump without data: @saas, my_log, tmp_* (--full dumps them with their data)\n",
		"Dumped without data (2 tables): cde_products_feed, my_log\n")
	mustContain(t, "the log", transferLogText(t), "Dumped without data:\n  cde_products_feed\n  my_log\n")
}

// TestTheLocalDumpRunsTheSameScript: without a tunnel, the same resolution in the db container.
func TestTheLocalDumpRunsTheSameScript(t *testing.T) {
	fake, _ := dbLab(t)
	answeringWith(t, "n")
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas,my_log"}

	transferDB([]string{"--sql-password=pw", "--structure-only=tmp_*"}, io.Discard, io.Discard, false)

	if got := steps(fake); len(got) == 0 || got[0] != "Capture db bash | -o | pipefail | -c | { "+structureOnlyScript(t)+" > /tmp/db.sql.gz; } 2>&1" {
		t.Fatalf("steps = %v, want the structure-only script first", got)
	}
}

// TestFullDumpsEveryTableWithItsData: --full is today's dump, whatever the project says.
func TestFullDumpsEveryTableWithItsData(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas,my_log"}

	var stdout bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw", "--structure-only=tmp_*", "--full"}, &stdout, io.Discard, false)

	if len(*ran) != 1 || (*ran)[0].Command() != tunnelledDump {
		t.Fatalf("ssh ran %v, want today's dump", *ran)
	}

	mustContain(t, "stdout", stdout.String(), "Dumping every table with its data (--full)\n")
	mustNotContain(t, "stdout", stdout.String(), "without data")
}

// TestWithoutRulesTheDumpIsTodays: a project whose TRANSFER_DB_STRUCTURE_ONLY is empty dumps as
// it always did, and says nothing new.
func TestWithoutRulesTheDumpIsTodays(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": ""}

	var stdout bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, io.Discard, false)

	if len(*ran) != 1 || (*ran)[0].Command() != tunnelledDump {
		t.Fatalf("ssh ran %v, want today's dump", *ran)
	}

	mustNotContain(t, "stdout", stdout.String(), "without data", "--full")
}

// TestRulesThatMatchNoTableSaySo: every table came with its data, which is what today's dump did.
func TestRulesThatMatchNoTableSaySo(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}
	sshAnswering(t, ran, sshOutcome{})

	var stdout bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, io.Discard, false)

	mustContain(t, "stdout", stdout.String(), "Dumped without data: no table matched, so every table has its data\n")
}

// TestManyStructureOnlyTablesAreListedInTheLog rather than flooding the terminal.
func TestManyStructureOnlyTablesAreListedInTheLog(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}

	tables := []string{}
	for at := range 12 {
		tables = append(tables, fmt.Sprintf("cde_feed_%d", at))
	}

	sshAnswering(t, ran, sshOutcome{said: structureOnlyLines(tables...)})

	var stdout bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, io.Discard, false)

	mustContain(t, "stdout", stdout.String(), "Dumped without data: 12 tables, listed in "+transferLogPath(t)+"\n")
	mustNotContain(t, "stdout", stdout.String(), "cde_feed_3")
	mustContain(t, "the log", transferLogText(t), "  cde_feed_0\n", "  cde_feed_11\n")
}

// TestAStructureOnlyFailureIsTheDumpsFailure: the structure pass reports its own failure, and
// stops the transfer before anything is copied, as the data pass does.
func TestAStructureOnlyFailureIsTheDumpsFailure(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}
	sshAnswering(t, ran, sshOutcome{said: structureOnlyLines("cde_products_feed") +
		"mariadb-dump: Couldn't find table: \"cde_products_feed\"\nhm-transfer-db: the structure-only dump exited with status 6\n"})

	var stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, true); code != 6 {
		t.Fatalf("transfer-db = %d, want the dump's 6", code)
	}

	if len(steps(fake)) != 0 {
		t.Fatalf("went on after the structure pass failed: %v", steps(fake))
	}

	_, message := failureIn(t, stderr.Bytes())
	mustContain(t, "the failure", message, "failed (the structure-only dump exited with status 6)", "Couldn't find table")
	mustNotContain(t, "the failure", message, "hm-transfer-db-structure-only:")
}

// TestAStructureOnlyDumpIsRetriedOn1412: the retry takes the whole script again, both passes.
func TestAStructureOnlyDumpIsRetriedOn1412(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}
	sshAnswering(t, ran, sshOutcome{said: structureOnlyLines("cde_products_feed") + tableChanged},
		sshOutcome{said: structureOnlyLines("cde_products_feed")})

	var stdout bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	if len(*ran) != 2 || (*ran)[1].Command() != (*ran)[0].Command() {
		t.Fatalf("ssh ran %d times, want the same script twice", len(*ran))
	}

	mustContain(t, "stdout", stdout.String(), "retrying (2/3)", "Dumped without data (1 table): cde_products_feed\n")
}

// TestAnUnknownRuleStopsBeforeAnything: a misspelt group would otherwise dump everything with its
// data, which is exactly what the rule was written to avoid.
func TestAnUnknownRuleStopsBeforeAnything(t *testing.T) {
	cases := []struct {
		name, property, option, says string
	}{
		{name: "in the project", property: "@saas,@nope", says: `unknown table group "@nope"`},
		{name: "in the run", property: "@saas", option: "--structure-only=a'b", says: `"a'b" is not a table name or pattern`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake, ran := dbLab(t)
			asked := answeringWith(t)
			fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": tt.property}

			args := []string{"--ssh-user=abc"}
			if tt.option != "" {
				args = append(args, tt.option)
			}

			var stderr bytes.Buffer

			if code := transferDB(args, io.Discard, &stderr, true); code != exitUsage {
				t.Fatalf("transfer-db = %d, want exitUsage\n%s", code, stderr.String())
			}

			if len(*ran) != 0 || len(steps(fake)) != 0 || len(*asked) != 0 {
				t.Fatalf("asked %v, ran %v and %v after refusing", *asked, *ran, steps(fake))
			}

			kind, message := failureIn(t, stderr.Bytes())
			if kind != "invalid_argument" {
				t.Errorf("failure type = %q, want invalid_argument", kind)
			}

			mustContain(t, "the failure", message, "TRANSFER_DB_STRUCTURE_ONLY or --structure-only", tt.says)
		})
	}
}

// TestTheStructureOnlyScriptRunsInEveryShell runs the script the transfer sends in each shell a
// server may log in with, against a fake client and a fake dump tool: the patterns match on the
// server, both passes land in one gzip stream, table names are passed as data whatever they
// contain, and each failure says which pass failed.
func TestTheStructureOnlyScriptRunsInEveryShell(t *testing.T) {
	fakes := t.TempDir()

	writeTool(t, fakes, "mariadb", `printf '%s\n' "$*" >> "$FAKE_CALLS"
[ -n "$FAKE_LIST_STATUS" ] && { echo "ERROR 1045 (28000): Access denied" >&2; exit "$FAKE_LIST_STATUS"; }
printf '%s\n' "$FAKE_TABLES"`)
	writeTool(t, fakes, "mariadb-dump", `printf 'dump'; for a; do printf ' [%s]' "$a"; done; printf '\n'
case " $* " in
*" --no-data "*) exit "${FAKE_STRUCTURE_STATUS:-0}" ;;
*) [ -n "$FAKE_DATA_SAYS" ] && echo "$FAKE_DATA_SAYS" >&2; exit "${FAKE_DATA_STATUS:-0}" ;;
esac`)

	tables := strings.Join([]string{"cde_products_feed", "catalog_product_entity", "sales_data_exporter_orders_v2_sync_batches",
		"my_log", "my log", "it's", "$(touch pwned)", "tmp_x", "tmp_it's $(touch pwned) x"}, "\n")

	options := `[--single-transaction] [--quick] [--no-tablespaces] [--skip-comments] [-hdatabase.internal] [-umysql] [-P] [3306] [-ppw]`

	cases := []struct {
		name    string
		env     []string
		dumped  string
		said    []string
		notSaid []string
	}{
		{
			name: "both passes",
			dumped: "dump " + options + " [--no-data] [main] [cde_products_feed] [sales_data_exporter_orders_v2_sync_batches] [my_log] [tmp_x] [tmp_it's $(touch pwned) x]\n" +
				"dump " + options + " [main] [--ignore-table=main.cde_products_feed] [--ignore-table=main.sales_data_exporter_orders_v2_sync_batches] [--ignore-table=main.my_log] [--ignore-table=main.tmp_x] [--ignore-table=main.tmp_it's $(touch pwned) x]\n",
			said: []string{"hm-transfer-db-structure-only: cde_products_feed\n", "hm-transfer-db-structure-only: tmp_x\n", "hm-transfer-db-structure-only: tmp_it's $(touch pwned) x\n"},
			notSaid: []string{"hm-transfer-db: ", "structure-only: catalog_product_entity", "structure-only: my log",
				"structure-only: it's", "structure-only: $(touch pwned)"},
		},
		{
			name:    "no table matches",
			env:     []string{"FAKE_TABLES=catalog_product_entity\nsales_order"},
			dumped:  "dump " + options + " [main]\n",
			notSaid: []string{"hm-transfer-db"},
		},
		{
			name:   "the structure pass fails",
			env:    []string{"FAKE_STRUCTURE_STATUS=6"},
			dumped: "dump " + options + " [--no-data] [main] [cde_products_feed] [sales_data_exporter_orders_v2_sync_batches] [my_log] [tmp_x] [tmp_it's $(touch pwned) x]\n",
			said:   []string{"hm-transfer-db: the structure-only dump exited with status 6\n"},
		},
		{
			name: "the data pass fails",
			env:  []string{"FAKE_DATA_STATUS=2", "FAKE_DATA_SAYS=mariadb-dump: Error 1412: Table definition has changed, please retry transaction when dumping table `sales_order` at row: 0"},
			said: []string{"Error 1412: Table definition has changed", "hm-transfer-db: the dump exited with status 2\n"},
		},
		{
			name:    "listing the tables fails",
			env:     []string{"FAKE_LIST_STATUS=1"},
			dumped:  "",
			said:    []string{"ERROR 1045 (28000): Access denied", "hm-transfer-db: listing the tables exited with status 1\n"},
			notSaid: []string{"structure-only:"},
		},
	}

	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas,my_log"}
	transferDB([]string{"--ssh-user=abc", "--sql-password=pw", "--structure-only=tmp_*"}, io.Discard, io.Discard, false)
	script := (*ran)[0].Command()

	for _, shell := range []string{"/bin/sh", "dash", "bash"} {
		path, err := exec.LookPath(shell)
		if err != nil {
			t.Logf("%s is not installed here", shell)

			continue
		}

		for _, tt := range cases {
			t.Run(filepath.Base(shell)+"/"+tt.name, func(t *testing.T) {
				work := t.TempDir()

				command := exec.Command(path, "-c", script)
				command.Dir = work
				command.Env = append([]string{"PATH=" + fakes + ":/usr/bin:/bin", "FAKE_CALLS=" + filepath.Join(work, "calls"),
					"FAKE_TABLES=" + tables}, tt.env...)

				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr

				if err := command.Run(); err != nil {
					t.Fatalf("the script failed by itself: %v\n%s", err, stderr.String())
				}

				dumped := ""
				if stdout.Len() > 0 {
					reader, err := gzip.NewReader(&stdout)
					if err != nil {
						t.Fatalf("stdout is not gzip: %v", err)
					}

					contents, _ := io.ReadAll(reader)
					dumped = string(contents)
				}

				if tt.name != "the data pass fails" && dumped != tt.dumped {
					t.Errorf("dumped\n%s\nwant\n%s", dumped, tt.dumped)
				}

				mustContain(t, "stderr", stderr.String(), tt.said...)
				mustNotContain(t, "stderr", stderr.String(), tt.notSaid...)

				if _, err := os.Stat(filepath.Join(work, "pwned")); err == nil {
					t.Fatal("a table name ran as a command")
				}

				calls, _ := os.ReadFile(filepath.Join(work, "calls"))
				mustContain(t, "the client's arguments", string(calls), "-N -B -r -e SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' main")
			})
		}
	}
}

func writeTool(t *testing.T, dir, name, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
}

// The rules given with a space (--structure-only my_log) are read like those given with "=",
// rather than ending up as an empty value and an ignored word: the tables would be dumped with
// their data although the run asked otherwise.
func TestStructureOnlyTakesItsValueAfterASpace(t *testing.T) {
	fake, ran := dbLab(t)
	fake.properties = map[string]string{"TRANSFER_DB_STRUCTURE_ONLY": "@saas"}
	sshAnswering(t, ran, sshOutcome{said: structureOnlyLines("my_log")})

	var stdout bytes.Buffer

	code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw", "--structure-only", "my_log", "--anonymise=false"},
		&stdout, io.Discard, false)
	if code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	mustContain(t, "stdout", stdout.String(), "Tables to dump without data: @saas, my_log (--full dumps them with their data)\n")
}

// --structure-only with nothing after it is a mistake, refused before anything is asked or run.
func TestStructureOnlyWithoutAValueIsRefused(t *testing.T) {
	for _, args := range [][]string{{"--ssh-user=abc", "--structure-only"}, {"--ssh-user=abc", "--structure-only="}} {
		fake, ran := dbLab(t)
		asked := answeringWith(t)

		var stderr bytes.Buffer

		if code := transferDB(args, io.Discard, &stderr, true); code != exitUsage {
			t.Fatalf("%v = %d, want exitUsage\n%s", args, code, stderr.String())
		}

		if len(*ran) != 0 || len(steps(fake)) != 0 || len(*asked) != 0 {
			t.Fatalf("%v: asked %v, ran %v and %v after refusing", args, *asked, *ran, steps(fake))
		}

		kind, message := failureIn(t, stderr.Bytes())
		if kind != "invalid_argument" || !strings.Contains(message, "--structure-only") {
			t.Errorf("%v: failure = %q %q, want invalid_argument naming --structure-only", args, kind, message)
		}
	}
}
