package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// structureOnlyProperty is the project's list of tables dumped without their data. Its default,
// in data/properties.json, is @saas: a project that says nothing still skips the data exporter's
// feeds and batches, which a live store alters while it is dumped.
const structureOnlyProperty = "TRANSFER_DB_STRUCTURE_ONLY"

// structureOnlyMarker starts the line the dump script prints on stderr for each table it dumps
// without data. It is not transferMarker: these lines are news, not failures.
const structureOnlyMarker = "hm-transfer-db-structure-only: "

// structureOnlyListed is how many structure-only tables are named on the terminal; past it they
// are counted there and named in the log.
const structureOnlyListed = 10

// structureOnlyPatterns is what rules stand for, from the groups the tool ships. --full stands
// for nothing: every table with its data.
func structureOnlyPatterns(rules []string, full bool) ([]string, error) {
	if full || len(rules) == 0 {
		return nil, nil
	}

	contents, err := fs.ReadFile(toolAssets(), "data/table-groups.json")
	if err != nil {
		return nil, err
	}

	groups, err := core.ParseTableGroups(contents)
	if err != nil {
		return nil, err
	}

	return core.StructureOnlyPatterns(groups, rules)
}

// structureOnlyDump is the dump in two passes into one gzip stream: the structure of the tables
// the patterns match, then every other table with its data. mariadb-dump's --ignore-table takes
// no wildcard, so the patterns are matched here, on the server, against the schema's tables as
// information_schema lists them: one script, so one SSH connection and one authentication.
//
// It is written for any POSIX shell, as dumpPipeline is. The matched names, which come from the
// server, are only ever expanded inside double quotes and collected as positional parameters —
// sh has no arrays — so a name with a space, a quote or a $( ) is passed as one argument and
// never run. The patterns are single-quoted values used as case patterns, where only their * is
// special: core.StructureOnlyPatterns let nothing else through. Each pass says when it failed,
// as each part of the pipeline does, and the data pass's message is the one the 1412 retry and
// the failure already read.
func structureOnlyDump(host, user, port, passwordFlag, database string, patterns []string) string {
	connection := fmt.Sprintf("-h'%s' -u'%s' -P '%s' %s", insideQuotes(host), insideQuotes(user), insideQuotes(port), passwordFlag)
	options := "--single-transaction --quick --no-tablespaces --skip-comments " + connection

	variables, cases := []string{}, []string{}
	for at, pattern := range patterns {
		variables = append(variables, fmt.Sprintf("p%d='%s'", at+1, insideQuotes(pattern)))
		cases = append(cases, fmt.Sprintf("$p%d", at+1))
	}

	return strings.Join([]string{
		`dump=$(command -v mariadb-dump || command -v mysqldump)`,
		`client=$(command -v mariadb || command -v mysql)`,
		`db='` + insideQuotes(database) + `'`,
		strings.Join(variables, "; "),
		// A newline for IFS, written without one: the whole script is one line
		`nl=$(printf '\n_'); nl=${nl%_}`,
		`if tables=$("$client" ` + connection + ` -N -B -r -e "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'" "$db"); then ` +
			`set --; ifs=$IFS; IFS=$nl; set -f; ` +
			`for t in $tables; do case $t in ` + strings.Join(cases, "|") + `) set -- "$@" "$t"; printf '` + structureOnlyMarker + `%s\n' "$t" >&2;; esac; done; ` +
			`IFS=$ifs; set +f; ` +
			`{ s=0; if [ $# -gt 0 ]; then "$dump" ` + options + ` --no-data "$db" "$@" || s=$?; fi; ` +
			`if [ $s -ne 0 ]; then echo "` + transferMarker + `the structure-only dump exited with status $s" >&2; ` +
			// Each matched table, rotated into its --ignore-table: the for list is read once
			`else for t do set -- "$@" "--ignore-table=$db.$t"; shift; done; ` +
			`"$dump" ` + options + ` "$db" "$@" || echo "` + transferMarker + `the dump exited with status $?" >&2; fi; } | ` +
			`{ sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' || echo "` + transferMarker + `sed exited with status $?" >&2; } | gzip -9; ` +
			`else echo "` + transferMarker + `listing the tables exited with status $?" >&2; fi`,
	}, "; ")
}

// changedTableIn is where error 1412 names its table: "... please retry transaction when dumping
// table `name` at row: 0", as mariadb-dump and mysqldump both word it.
var changedTableIn = regexp.MustCompile("when dumping table `([^`]+)`")

// changedTable is the table the last error 1412 of a dump named, or nothing.
func changedTable(said string) string {
	found := changedTableIn.FindAllStringSubmatch(said, -1)
	if len(found) == 0 {
		return ""
	}

	return found[len(found)-1][1]
}

// structureOnlySuggestion is what to do about a table the database kept changing under the dump:
// the exact line for config/docker/properties.json that dumps it without data, the project's own
// list kept in front of it (the run's --structure-only is not the project's, so it is left out).
// Nothing when there is no table, or when the rules already take it without data and the failure
// came from somewhere else; under --full, that the rules already name it.
func structureOnlySuggestion(table, projectRules string, rules []string, full bool) string {
	// A name the rules could not carry would make a line the next run refuses
	if _, err := core.StructureOnlyPatterns(nil, []string{table}); table == "" || err != nil {
		return ""
	}

	if patterns, err := structureOnlyPatterns(rules, false); err == nil {
		for _, pattern := range patterns {
			if !core.TableMatches(pattern, table) {
				continue
			}

			if full {
				return "`" + table + "` is in " + structureOnlyProperty + " already: run without --full to dump it without its data"
			}

			return ""
		}
	}

	value := strings.Join(append(core.SplitTableRules(projectRules), table), ",")

	key, _ := json.Marshal(structureOnlyProperty)
	quoted, _ := json.Marshal(value)

	return "To dump `" + table + "` without its data from now on, set this in config/docker/properties.json:\n" +
		string(key) + ": " + string(quoted)
}

// dumpedWithoutData is the tables the dump script said it took without data, in its order.
func dumpedWithoutData(said string) []string {
	tables := []string{}

	for _, line := range strings.Split(said, "\n") {
		if table, found := strings.CutPrefix(strings.TrimRight(line, "\r"), structureOnlyMarker); found {
			tables = append(tables, table)
		}
	}

	return tables
}

// reportStructureOnly says which tables came without their data: by name when they are few, by
// count and the log's path when they are many. The log always names every one.
func reportStructureOnly(tables []string, log *transferLog, say func(string)) {
	if len(tables) == 0 {
		say("Dumped without data: no table matched, so every table has its data\n")

		return
	}

	log.printf("Dumped without data:\n  %s\n", strings.Join(tables, "\n  "))

	noun := "tables"
	if len(tables) == 1 {
		noun = "table"
	}

	if len(tables) > structureOnlyListed && log.path != "" {
		say(fmt.Sprintf("Dumped without data: %d %s, listed in %s\n", len(tables), noun, log.path))

		return
	}

	say(fmt.Sprintf("Dumped without data (%d %s): %s\n", len(tables), noun, strings.Join(tables, ", ")))
}
