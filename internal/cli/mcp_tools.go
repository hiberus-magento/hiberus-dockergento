package cli

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

//
// The tools, each a wrapper over a command of this binary that already answers, so there is one
// implementation of "what is this project" and it is the one with tests. The exception is the
// database tool, which takes a string from a model and gives it to a database: that one needs a
// rule, and the rule is mcpQueryRefusal.
//

// call runs one tool and answers with its content.
func (s mcpServer) call(name string, arguments jqValue) jqValue {
	if mcpTakesEnvironment(name, s.write) {
		dir, refusal := s.environment(rawField(arguments, "environment", ""))
		if refusal != "" {
			return mcpContent(refusal, true)
		}

		s.dir = dir
	}

	switch name {
	case "describe_project":
		return s.fromCommand("describe")
	case "list_environments":
		return s.fromCommand("list")
	case "check_environment":
		return s.fromCommand("doctor")
	case "environment_status":
		return s.fromCommand("status")
	case "xdebug":
		return s.xdebug(arguments)
	case "after_change":
		return s.afterChange(arguments)
	case "magento_exceptions":
		return s.magentoExceptions(arguments)
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
		case "run_tests":
			return s.runTests(arguments)
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
	return substituted(string(s.capture(s.dir, args, combined)))
}

// mcpTakesEnvironment says whether a tool answers about one environment, and so takes the
// `environment` argument: every tool there is but list_environments, which is about the machine.
func mcpTakesEnvironment(name string, write bool) bool {
	if name == "list_environments" {
		return false
	}

	for _, tool := range mcpToolNames(write) {
		if tool == name {
			return true
		}
	}

	return false
}

// environment is the directory of the branch environment called name, or why there is none.
// Nothing at all is the directory the server runs in, as before the argument existed.
//
// The registry is asked through `worktree list`, the command that already answers which branch
// environments this project has and where they are, so a name means here what it means there.
// The directory comes from the registry, never from the model: a name is only ever matched.
func (s mcpServer) environment(name string) (string, string) {
	if name == "" {
		return "", ""
	}

	output := s.run(false, "worktree", "list", "--json")

	document, ok := parseJQ([]byte(output))
	data, _ := document.field("data")
	listed, _ := data.field("worktrees")

	if !ok || listed.kind != jqArray {
		return "", "The branch environments of this project could not be read, so '" + name +
			"' cannot be found. Leave environment out to use the current directory, or try check_environment."
	}

	var names []string

	for _, worktree := range listed.items {
		called := rawField(worktree, "name", "")
		names = append(names, called)

		if called != name {
			continue
		}

		path := rawField(worktree, "path", "")

		if rawField(worktree, "state", "") == "missing" || path == "" {
			return "", "The branch environment '" + name + "' no longer has its directory (" + path +
				"). Collect it with " + binaryName() + " clean."
		}

		return path, ""
	}

	if len(names) == 0 {
		return "", "This project has no branch environments, so there is none called '" + name +
			"'. Leave environment out to use the current directory."
	}

	return "", "There is no branch environment called '" + name + "' in this project. The ones there are: " +
		strings.Join(names, " ")
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

	content := mcpContent(substituted(data.raw()), false)

	//
	// The same data again as structuredContent, which a client can validate against the tool's
	// outputSchema and read without parsing text. Only an object: that is what the schemas
	// declare, and what a 2025-06-18 client accepts there. The text stays for the clients that
	// read nothing else.
	//
	if data.kind == jqObject {
		content = content.with("structuredContent", data)
	}

	return content
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

// mcpReadOnly goes before every statement database_query runs: the second layer, for what the
// text guard lets through and the statement still does — a SELECT calling a stored function that
// writes, a locking read. The database refuses those itself inside a read-only transaction.
//
// The session setting as well as the transaction: a statement that commits implicitly ends the
// transaction, and what runs after it starts another in the session's mode, which is read-only too.
// The statement goes last and on its own line, with no ROLLBACK after it, because a trailing
// `-- comment` would swallow anything put after it on that line and a trailing ';' would leave an
// empty statement, which the client reports as an error. The client closes the connection when it
// is done, and a transaction that could not write has nothing to roll back.
const mcpReadOnly = "SET SESSION TRANSACTION READ ONLY;\nSTART TRANSACTION READ ONLY;\n"

func (s mcpServer) databaseQuery(arguments jqValue) jqValue {
	sql := rawField(arguments, "sql", "")

	if refusal := mcpQueryRefusal(sql); refusal != "" {
		return mcpContent(refusal, true)
	}

	output, status := s.statusOf(true, "--no-json", "mysql", "-q", mcpReadOnly+sql)

	// What the database refused is an error, in its own words: the read-only transaction is only a
	// second layer if a statement it stops reaches the model as a refusal, not as the rows asked for
	refused := status != 0 || mysqlError.MatchString(output)

	if output == "" {
		if refused {
			return mcpContent("The database client exited with status "+strconv.Itoa(status)+" and said nothing.", true)
		}

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

	return mcpContent(output, refused)
}

// mysqlError is the line the MariaDB and MySQL clients print when the server refuses a statement:
// `ERROR 1792 (25006) at line 3: ...`, at the start of a line. Batch mode prints rows tab-separated
// and unadorned, so a row would have to be exactly that to be mistaken for one.
var mysqlError = regexp.MustCompile(`(?m)^ERROR \d+( \([0-9A-Z]{5}\))?( at line \d+)?:`)

// magento is one Magento command with its arguments checked. Together the write tools are a
// smaller permission than the shell an agent is given today to do the same four things.
func (s mcpServer) magento(args ...string) jqValue {
	command := append([]string{"--no-json", "magento"}, args...)

	var output string
	if s.progress != nil {
		output = s.followed(command...)
	} else {
		output = s.run(true, command...)
	}

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

var configPath = regexp.MustCompile(`^[a-z0-9_]+(/[a-z0-9_]+)+$`)

const shellSpace = " \t\n\v\f\r"

// mcpQueryRefusal says why a statement cannot be run, or nothing when it only reads.
//
// A whitelist, and one that errs towards refusing. A model that cannot run its query gets a
// sentence explaining why; a model that can run `SELECT ... INTO OUTFILE` gets a file written on
// the host as the database user.
func mcpQueryRefusal(sql string) string {
	//
	// Comments first: everything after them is invisible to the checks below and visible to the
	// database, which is the whole trick. Read as the server reads them — a '#' or '-- ' inside a
	// quoted string is text, not a comment — and a comment the server executes is refused
	// outright. The shell's sed took them line by line without seeing strings: a deliberate
	// difference, because that let a quoted '#' hide the rest of the statement.
	//
	code, executable, ambiguous := sqlWithoutComments(sql)

	//
	// Whether a backslash in '...' or "..." escapes the quote after it is the server's
	// NO_BACKSLASH_ESCAPES (and ANSI_QUOTES for "..."), which cannot be seen from here. Read the
	// other way, the string ends somewhere else, and a comment, an executable comment, a ';' or a
	// file operation can sit where this reading sees text. No reading can be trusted, so none is
	// attempted: such a statement is refused whatever follows (the shell did not see strings at
	// all — a deliberate difference).
	//
	if ambiguous {
		return "Refused: a backslash in a quoted string is read differently depending on the server's SQL mode " +
			"(NO_BACKSLASH_ESCAPES), so the statement cannot be checked. Write it without backslashes: " +
			"double a quote inside a string ('it''s'), use CHAR(92) for a literal backslash, " +
			"and give LIKE another escape character (LIKE '%!_%' ESCAPE '!')"
	}

	if executable {
		return "Refused: a /*! comment is run by the database, so what is in it cannot be checked"
	}

	// Any run of space is one space, so the words below are found however they are spaced
	stripped := strings.Join(strings.Fields(code), " ")

	// A single trailing semicolon is ordinary
	stripped = strings.TrimRight(strings.TrimSuffix(stripped, ";"), shellSpace)

	if stripped == "" {
		return "There is no statement to run"
	}

	if strings.Contains(stripped, ";") {
		return "One statement at a time: this contains more than one"
	}

	upper := strings.ToUpper(stripped)

	// Also asked of the statement as typed with everything but letters, digits and _ taken out,
	// so no way of reading its comments and strings — a server with NO_BACKSLASH_ESCAPES reads
	// them differently — can hide these words. A column called intooutfile is refused with them.
	compact := strings.Map(func(r rune) rune {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToUpper(r)
		}

		return -1
	}, sql)

	for _, file := range []string{"INTO OUTFILE", "INTO DUMPFILE", "LOAD_FILE"} {
		if strings.Contains(upper, file) || strings.Contains(compact, strings.ReplaceAll(file, " ", "")) {
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

// sqlWithoutComments is the statement with each comment replaced by a space (a block comment
// with a * in it kept whole) and its quoted strings left as they are, whether it holds a comment
// the server executes (/*! or /*M!), and whether a '...' or "..." string holds a backslash — where
// the server's SQL mode decides where the string ends.
// Inside '...' and "..." a backslash escapes the next character, as MySQL reads them by default;
// a doubled quote needs nothing, being a string closed and opened again.
func sqlWithoutComments(sql string) (string, bool, bool) {
	var out strings.Builder

	ambiguous := false

	text := []rune(sql)

	for at := 0; at < len(text); at++ {
		char := text[at]
		next := rune(0)

		if at+1 < len(text) {
			next = text[at+1]
		}

		switch {
		case char == '\'' || char == '"' || char == '`':
			end := at + 1
			for ; end < len(text) && text[end] != char; end++ {
				if text[end] == '\\' && char != '`' {
					ambiguous = true
					end++
				}
			}

			end = min(end, len(text)-1)
			out.WriteString(string(text[at : end+1]))
			at = end
		case char == '/' && next == '*':
			if rest := string(text[at+2:]); strings.HasPrefix(rest, "!") || strings.HasPrefix(rest, "M!") {
				return "", true, ambiguous
			}

			end := at + 2
			for ; end+1 < len(text) && !(text[end] == '*' && text[end+1] == '/'); end++ {
			}

			// One with a * inside stays, as the shell's /\*[^*]*\*/ left it, for the checks to
			// see and refuse: never less strict than the shell
			if body := text[at+2 : min(end, len(text))]; strings.ContainsRune(string(body), '*') {
				out.WriteString(string(text[at:min(end+2, len(text))]))
			} else {
				out.WriteRune(' ')
			}

			at = end + 1
		case char == '#', char == '-' && next == '-' && at+2 < len(text) && unicode.IsSpace(text[at+2]):
			for at < len(text) && text[at] != '\n' {
				at++
			}

			out.WriteRune(' ')
		default:
			out.WriteRune(char)
		}
	}

	return out.String(), false, ambiguous
}

// mcpIsConfigPath says whether this looks like a Magento configuration path.
//
// Not a boundary against a hostile model, and not pretending to be one: it is what stops a
// plausible mistake from being passed on as a command argument. Asked of the whole value: the
// shell's grep asked each line, so one good line let any other through with it.
func mcpIsConfigPath(path string) bool {
	return configPath.MatchString(path)
}
