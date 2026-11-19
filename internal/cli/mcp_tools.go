package cli

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

//
// The tools, each a wrapper over a command of this binary that already answers, so there is one
// implementation of "what is this project" and it is the one with tests. The exception is the
// database tool, which takes a string from a model and gives it to a database: that one needs a
// rule, and the rule is mcpQueryRefusal.
//

// call runs one tool and answers with its content.
func (s mcpServer) call(name string, arguments jqValue) jqValue {
	switch name {
	case "describe_project":
		return s.fromCommand("describe")
	case "list_environments":
		return s.fromCommand("list")
	case "check_environment":
		return s.fromCommand("doctor")
	case "service_logs":
		return s.serviceLogs(arguments)
	case "database_query":
		return s.databaseQuery(arguments)
	}

	//
	// Only reachable when the server was started with --write. Without it these names are not in
	// the catalogue either, so a model never sees them and has nothing to plan around.
	//
	if s.write {
		switch name {
		case "cache_flush":
			return s.magento("cache:flush")
		case "cache_clean":
			return s.magento(append([]string{"cache:clean"}, cacheTypes(arguments)...)...)
		case "reindex":
			if indexer := rawField(arguments, "indexer", ""); indexer != "" {
				return s.magento("indexer:reindex", indexer)
			}

			return s.magento("indexer:reindex")
		case "config_set":
			path := rawField(arguments, "path", "")
			value := rawField(arguments, "value", "")

			if !mcpIsConfigPath(path) {
				return mcpContent("'"+path+"' is not a configuration path. They look like section/group/field, for example web/secure/use_in_frontend.", true)
			}

			return s.magento("config:set", path, value)
		}
	}

	names := ""
	for _, tool := range mcpToolNames(s.write) {
		names += tool + " "
	}

	return mcpContent("There is no tool called '"+name+"'. The ones there are: "+names, true)
}

// run is one command of this binary, its output as a command substitution would keep it.
func (s mcpServer) run(combined bool, args ...string) string {
	return substituted(string(s.capture(args, combined)))
}

// fromCommand asks a command for its JSON and gives the model the data: the `ok` and
// `schema_version` scaffolding stays between the commands. Its standard error is dropped.
func (s mcpServer) fromCommand(command string) jqValue {
	output := s.run(false, command, "--json")

	document, ok := parseJQ([]byte(output))
	if output == "" || !ok || !document.truthy() {
		return mcpContent(s.executable+" "+command+" produced nothing readable. The environment may not be running: try check_environment.", true)
	}

	//
	// The shell asked `.ok // true` whether this was a failure, and that is true for `ok: false`
	// as well, so a failure document reached the model whole, as data. Kept, because parity is the
	// requirement here; the commands put their failures on stderr in any case, which is dropped.
	//
	if document.kind != jqObject {
		// jq stopped on `.data` and printed nothing
		return mcpContent("", false)
	}

	data, _ := document.field("data")
	if !data.truthy() {
		data = document
	}

	return mcpContent(substituted(data.raw()), false)
}

func (s mcpServer) serviceLogs(arguments jqValue) jqValue {
	service := rawField(arguments, "service", "")
	lines := rawField(arguments, "lines", "100")

	if service == "" {
		return mcpContent("Which service? Try phpfpm, nginx, db, search or redis.", true)
	}

	if lines == "" || strings.Trim(lines, "0123456789") != "" {
		lines = "100"
	}

	limit := os.Getenv("HM_MCP_LOG_CAP")
	if limit == "" {
		limit = "500"
	}

	if greater(lines, limit) {
		lines = limit
	}

	//
	// The global flags go before the command name: `logs` and `mysql` pass what follows them
	// through to Compose and to the client, where `--no-json` is not a flag anybody knows
	//
	output := s.run(true, "--no-json", "logs", "--tail", lines, service)
	if output == "" {
		output = "(the log is empty)"
	}

	return mcpContent(output, false)
}

func (s mcpServer) databaseQuery(arguments jqValue) jqValue {
	sql := rawField(arguments, "sql", "")

	if refusal := mcpQueryRefusal(sql); refusal != "" {
		return mcpContent(refusal, true)
	}

	output := s.run(true, "--no-json", "mysql", "-q", sql)
	if output == "" {
		return mcpContent("(no rows)", false)
	}

	limit := os.Getenv("HM_MCP_ROW_CAP")
	if limit == "" {
		limit = "200"
	}

	rows := strings.Split(output, "\n")
	if greater(strconv.Itoa(len(rows)), limit) {
		// head -n refuses a negative count and prints nothing
		kept := ""
		if keep, _ := strconv.Atoi(limit); keep >= 0 {
			kept = strings.Join(rows[:keep], "\n")
		}

		output = kept +
			"\n... truncated at " + limit + " rows. Narrow the query with a WHERE or a LIMIT."
	}

	return mcpContent(output, false)
}

// magento is one Magento command with its arguments checked. Together the write tools are a
// smaller permission than the shell an agent is given today to do the same four things.
func (s mcpServer) magento(args ...string) jqValue {
	output := s.run(true, append([]string{"--no-json", "magento"}, args...)...)
	if output == "" {
		output = "(done)"
	}

	return mcpContent(output, false)
}

// greater is `[ "$a" -gt "$b" ]`: false, as the test's error is, when either is not a number.
func greater(a, b string) bool {
	x, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return false
	}

	y, err := strconv.ParseInt(b, 10, 64)
	if err != nil {
		return false
	}

	return x > y
}

// cacheTypes is `(.types // []) | join(" ")`, split into words the way the shell split it
// unquoted.
func cacheTypes(arguments jqValue) []string {
	types, ok := arguments.field("types")
	if !ok || !types.truthy() {
		return nil
	}

	if types.kind != jqArray && types.kind != jqObject {
		return nil
	}

	words := make([]string, 0, len(types.items))
	for _, item := range types.items {
		switch item.kind {
		case jqString:
			words = append(words, item.text)
		case jqNull:
			words = append(words, "")
		case jqArray, jqObject:
			// jq refuses to join those, and the shell went on with nothing
			return nil
		default:
			words = append(words, item.compact())
		}
	}

	return strings.FieldsFunc(strings.Join(words, " "), func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n'
	})
}

var (
	sqlBlockComment = regexp.MustCompile(`/\*[^*]*\*/`)
	sqlDashComment  = regexp.MustCompile(`--[[:space:]].*$`)
	sqlHashComment  = regexp.MustCompile(`#.*$`)
	configPath      = regexp.MustCompile(`^[a-z0-9_]+(/[a-z0-9_]+)+$`)
)

const shellSpace = " \t\n\v\f\r"

// mcpQueryRefusal says why a statement cannot be run, or nothing when it only reads.
//
// A whitelist, and one that errs towards refusing. A model that cannot run its query gets a
// sentence explaining why; a model that can run `SELECT ... INTO OUTFILE` gets a file written on
// the host as the database user.
func mcpQueryRefusal(sql string) string {
	//
	// Comments first: everything after them is invisible to the checks below and visible to the
	// database, which is the whole trick. One line at a time, as sed read them.
	//
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		line = sqlBlockComment.ReplaceAllString(line, " ")
		line = sqlDashComment.ReplaceAllString(line, "")
		lines[i] = sqlHashComment.ReplaceAllString(line, "")
	}

	stripped := strings.ReplaceAll(strings.Join(lines, " "), "\t", " ")

	// Leading and trailing space, and a single trailing semicolon, are ordinary
	stripped = strings.Trim(stripped, shellSpace)
	stripped = strings.TrimRight(strings.TrimSuffix(stripped, ";"), shellSpace)

	if stripped == "" {
		return "There is no statement to run"
	}

	if strings.Contains(stripped, ";") {
		return "One statement at a time: this contains more than one"
	}

	upper := strings.ToUpper(stripped)

	for _, file := range []string{"INTO OUTFILE", "INTO DUMPFILE", "LOAD_FILE"} {
		if strings.Contains(upper, file) {
			return "Refused: that writes or reads a file as the database user"
		}
	}

	if strings.HasPrefix(upper, "SELECT(") {
		return ""
	}

	for _, verb := range []string{"SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN"} {
		if len(upper) > len(verb) && strings.HasPrefix(upper, verb) &&
			strings.ContainsRune(shellSpace, rune(upper[len(verb)])) {
			return ""
		}
	}

	return "Only SELECT, SHOW, DESCRIBE and EXPLAIN can be run here"
}

// mcpIsConfigPath says whether this looks like a Magento configuration path.
//
// Not a boundary against a hostile model, and not pretending to be one: it is what stops a
// plausible mistake from being passed on as a command argument. Asked of each line, as grep did.
func mcpIsConfigPath(path string) bool {
	if path == "" || strings.HasSuffix(path, "/") || strings.HasPrefix(path, "/") ||
		!strings.Contains(path, "/") {
		return false
	}

	for _, line := range strings.Split(path, "\n") {
		if configPath.MatchString(line) {
			return true
		}
	}

	return false
}
