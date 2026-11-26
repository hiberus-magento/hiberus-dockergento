package cli

import (
	"fmt"
	"io/fs"
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
