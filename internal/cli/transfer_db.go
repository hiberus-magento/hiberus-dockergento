package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"golang.org/x/term"
)

// The Magento Cloud defaults transfer-db.sh offered: only the SSH user and the password have none.
const (
	defaultDBSSHHost = "ssh.eu-3.magento.cloud"
	defaultDBHost    = "database.internal"
	defaultDBPort    = "3306"
	defaultDBUser    = "mysql"
	defaultDBName    = "main"
)

// runSSH runs ssh with its output into a file, what it and the remote command complain about
// into complaints, and stdin attached to this terminal. A passphrase, a host key or a second factor
// is asked on /dev/tty, not on stderr, so capturing stderr keeps interactive authentication
// working. It answers ssh's own exit status. A package variable so that no test ever connects
// anywhere.
var runSSH = func(args []string, output string, complaints io.Writer) (int, error) {
	file, err := os.Create(output) //nolint:gosec
	if err != nil {
		return 0, err
	}
	defer file.Close()

	command := exec.Command("ssh", args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, file, complaints

	err = command.Run()

	var exited *exec.ExitError
	if errors.As(err, &exited) {
		return exited.ExitCode(), nil
	}

	return 0, err
}

// askTransfer and readSecret are the questions, replaceable so that a test can answer them in
// order without a terminal.
var askTransfer = ask

// readSecret asks without echo when there is a terminal: it is a client environment's database
// password, and it would otherwise stay on screen and in the scrollback.
var readSecret = func(text string) (string, error) {
	fmt.Fprint(os.Stderr, prompt(text+" ", ""))

	if isTerminal(os.Stdin) {
		answer, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec

		// Without echo the Enter leaves no newline of its own
		fmt.Fprint(os.Stderr, "\n")

		return strings.TrimSpace(string(answer)), err
	}

	answer, _ := readLine(os.Stdin)

	return strings.TrimSpace(answer), nil
}

// transferDB brings a remote database into this project's — transfer-db.sh, questions, defaults,
// messages and commands included: a dump taken over SSH (or from the db container itself when
// there is no tunnel), restored keeping this machine's core_config_data and admin users if asked,
// optionally anonymised, then Magento's config import, reindex and cache flush.
func transferDB(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	sshHost, sshUser := defaultDBSSHHost, ""
	sqlHost, sqlPort, sqlUser, sqlDB, sqlPassword := defaultDBHost, defaultDBPort, defaultDBUser, defaultDBName, ""
	anonymise := "prompt"

	// Anything else is ignored, as the shell's catch-all case ignored it
	for _, argument := range args {
		name, value, _ := strings.Cut(argument, "=")

		switch name {
		case "--ssh-host":
			sshHost = value
		case "--ssh-user":
			sshUser = value
		case "--sql-host":
			sqlHost = value
		case "--sql-port":
			sqlPort = value
		case "--sql-user":
			sqlUser = value
		case "--sql-db":
			sqlDB = value
		case "--sql-password":
			sqlPassword = value
		case "--anonymise":
			anonymise = value
		}
	}

	project, code := projectOr(stderr, jsonOutput, "transfer-db")
	if code != 0 {
		return code
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	description, err := engine.Describe(project.Root, false)
	if err != nil {
		return report(stderr, jsonOutput, "transfer-db", err)
	}

	for _, service := range []string{phpService, "db"} {
		if !serviceRunning(description, service) {
			return failure(stderr, jsonOutput, "transfer-db", exitService, "service_not_running",
				fmt.Sprintf("Service '%s' is not running", service), binaryName()+" start "+service)
		}
	}

	// print_info: green on stdout, plain on stderr under --json, where stdout is the containers'
	say := func(text string) {
		if jsonOutput {
			fmt.Fprint(stderr, text)

			return
		}

		fmt.Fprint(stdout, good(text))
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	// question is asked only with somebody to answer; without, the answer is empty, and an empty
	// answer is what falls back to the option or the default
	question := func(text, suggestion string) string {
		if !interactive {
			return ""
		}

		answer, err := askTransfer(text, suggestion)
		if err != nil || answer == suggestion {
			return ""
		}

		return answer
	}

	yes := func(answer string) bool { return answer == "" || answer == "Y" || answer == "y" }
	or := func(answer, fallback string) string {
		if answer != "" {
			return answer
		}

		return fallback
	}

	say("Database transfer assistant: \n")

	if yes(question("Do you need to use SSH tunneling? [Y/n]:", "")) {
		host := question("SSH Host", sshHost)
		user := question("SSH User", sshUser)
		sshHost, sshUser = or(host, sshHost), or(user, sshUser)
	} else {
		sshHost, sshUser = "", ""
	}

	host := question("Database Host", sqlHost)
	port := question("Database Port", sqlPort)
	user := question("Database User", sqlUser)
	name := question("Database DB Name", sqlDB)

	password := ""
	if interactive {
		// The default is not shown: the shell printed the password passed as an option
		password, _ = readSecret("Database Password")
	}

	sqlHost, sqlPort, sqlUser, sqlDB = or(host, sqlHost), or(port, sqlPort), or(user, sqlUser), or(name, sqlDB)
	sqlPassword = or(password, sqlPassword)

	passwordFlag := ""
	if sqlPassword != "" {
		passwordFlag = "-p'" + insideQuotes(sqlPassword) + "'"
	}

	exclude := yes(question("Do you want to exclude 'core_config_data' table? [Y/n]:", ""))

	say(fmt.Sprintf("You are going to transfer database from [%s:[%s:%s]] to [LOCALHOST].\n", sshHost, sqlHost, sqlPort))
	question("Press any key continue...", "")

	if sqlHost == "" || sqlPort == "" || sqlUser == "" || sqlDB == "" {
		return failure(stderr, jsonOutput, "transfer-db", exitUsage, "input_required",
			"Missing connection data for the remote database", binaryName()+" transfer-db --help")
	}

	say("Creating database dump from origin server...\n")

	dump := dumpPipeline(sqlHost, sqlUser, sqlPort, passwordFlag, sqlDB)

	// fail stops the transfer with what failed and why, and the last of what the step said
	fail := func(code int, headline, said string) int {
		message := headline
		if tail := stepTail(said, transferTailLines); tail != "" {
			message += ":\n" + tail
		}

		if code == 0 {
			code = exitError
		}

		return failure(stderr, jsonOutput, "transfer-db", code, "transfer_failed", message, "")
	}

	// broke stops the transfer at a step that could not even be run, keeping the refusal's kind
	broke := func(step string, err error) int {
		var refused core.Refusal
		if asRefusal(err, &refused) {
			return failure(stderr, jsonOutput, "transfer-db", refused.Code, refused.Kind,
				step+" failed: "+refused.Message, refused.Hint)
		}

		return failure(stderr, jsonOutput, "transfer-db", exitDocker, "docker_failed", step+" failed: "+err.Error(), "")
	}

	// inDB runs one step in the db container, under bash with pipefail, both of its streams
	// captured: what it said is what a failure quotes
	inDB := func(command string) (string, int, error) {
		return engine.Capture(project.Root, "db", []string{"bash", "-o", "pipefail", "-c", command})
	}

	step, runner := "Creating the database dump in the db container", ""
	local := filepath.Join(here(), "db.sql.gz")

	if sshHost != "" {
		step, runner = "Creating the database dump on "+sshHost, "ssh"
	}

	// takeDump runs the dump once, and answers what it said and how it ended
	takeDump := func() (string, int, error) {
		if sshHost == "" {
			return inDB("{ " + dump + " > /tmp/db.sql.gz; } 2>&1")
		}

		var said strings.Builder

		status, err := runSSH([]string{sshUser + "@" + sshHost, dump}, local, &said)

		return said.String(), status, err
	}

	for attempt := 1; ; attempt++ {
		said, status, err := takeDump()

		switch {
		case err != nil && runner == "ssh":
			return failure(stderr, jsonOutput, "transfer-db", exitError, "transfer_failed",
				"The database dump could not be taken over SSH: "+err.Error(), "")
		case err != nil:
			return broke(step, err)
		}

		why, code, failed := stepFailed(said, status, runner)
		if !failed {
			break
		}

		// Only the dump is taken again: nothing after it has run yet
		if attempt < transferDumpAttempts && askedToRetry(said) {
			say(fmt.Sprintf("The table definition changed during the dump, retrying (%d/%d)...\n",
				attempt+1, transferDumpAttempts))

			continue
		}

		headline := step + " failed (" + why + ")"
		if attempt > 1 {
			headline = fmt.Sprintf("%s failed after %d attempts (%s)", step, attempt, why)
		}

		return fail(code, headline, said)
	}

	if sshHost != "" {
		if err := engine.CopyIntoService(project.Root, "db", local, "/tmp"); err != nil {
			return broke("Copying the dump into the db container", err)
		}
	}

	say("Restoring database dump into localhost...\n")

	// inDBOr runs a step in the db container and stops the transfer if it fails
	inDBOr := func(step, command string) int {
		said, status, err := inDB(command)
		if err != nil {
			return broke(step, err)
		}

		if why, code, failed := stepFailed(said, status, ""); failed {
			return fail(code, step+" failed ("+why+")", said)
		}

		return exitOK
	}

	kept := "core_config_data, admin_user and tfa_user_config"

	if exclude {
		if code := inDBOr("Saving "+kept, keepConfigCommand); code != exitOK {
			return code
		}
	}

	restoring := "Restoring the dump into the local database"

	said, status, err := inDB(restoreCommand)
	if err != nil {
		return broke(restoring, err)
	}

	// --force goes on past a statement this server refuses, and the client then exits with a
	// status; that is what was asked for. A status with nothing skipped, or a part of the pipeline
	// that failed, is a restore that did not happen
	skipped := skippedErrors(said)
	forced := status != 0 && len(skipped) > 0 && !strings.Contains(said, transferMarker)

	if why, code, failed := stepFailed(said, status, ""); failed && !forced {
		return fail(code, restoring+" failed ("+why+")", said)
	}

	if exclude {
		if code := inDBOr("Putting "+kept+" back", putConfigBackCommand); code != exitOK {
			return code
		}
	}

	// A database from another environment has just replaced this one: whatever was true about
	// the old contents is not true about these
	if err := engine.ClearAnonymisation(project.Root); err != nil {
		return broke("Recording that the database is no longer anonymised", err)
	}

	switch {
	case anonymise == "true":
		anonymiseOrWarn(engine, project.Root, stderr)
	case anonymise == "false":
		say("Skipping database anonymisation...\n")
	case yes(question("Do you want to anonymise the database? [Y/n]:", "")):
		anonymiseOrWarn(engine, project.Root, stderr)
	default:
		say("Skipping database anonymisation...\n")
	}

	for _, step := range []struct{ question, command string }{
		{"Do you want to import app config (app:config:import)? [Y/n]:", "app:config:import"},
		{"Do you want to reindex Magento? [Y/n]:", "indexer:reindex"},
		{"Do you want to clear Magento cache? [Y/n]:", "cache:flush"},
	} {
		if !yes(question(step.question, "")) {
			continue
		}

		running := "Running bin/magento " + step.command

		status, err := engine.Exec(project.Root, phpService, []string{"bin/magento", step.command}, terminalOptions(""))

		switch {
		case status != 0:
			return fail(status, fmt.Sprintf("%s failed (it exited with status %d; its output is above)", running, status), "")
		case err != nil:
			return broke(running, err)
		}
	}

	if len(skipped) > 0 {
		noun := "errors"
		if len(skipped) == 1 {
			noun = "error"
		}

		fmt.Fprint(stderr, warning(fmt.Sprintf("The restore skipped %d SQL %s; the first: %s\n", len(skipped), noun, skipped[0])))
	}

	say(" All done!\n")

	return exitOK
}

// anonymiseOrWarn anonymises, and says so when it could not without stopping: the shell's
// `masquerade_run && hm_anonymisation_record` never stopped the transfer either. The engine
// announces the anonymisation itself, and records it only when it worked.
func anonymiseOrWarn(engine commands, dir string, stderr io.Writer) {
	if err := engine.Anonymise(dir); err != nil {
		fmt.Fprint(stderr, warning("The database was not anonymised: "+err.Error()+"\n"))
	}
}

// transferTailLines is how much of a failed step's output a failure quotes: the reason is
// almost always the last line, and the few before it say where.
const transferTailLines = 10

// The steps in the db container. Each says what went wrong on its stdout, where Capture reads
// it: the core_config_data dump used to send its errors to /dev/null, and the restore to the
// terminal, where nothing kept them.
const (
	keepConfigCommand = `dump=$(command -v mariadb-dump || command -v mysqldump); { "$dump" -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE core_config_data admin_user tfa_user_config > /tmp/ccd.sql; } 2>&1`

	restoreCommand = `client=$(command -v mariadb || command -v mysql); { { zcat /tmp/db.sql.gz || echo "` + transferMarker + `zcat exited with status $?" >&2; } | ` +
		`{ sed '/sandbox mode/d' || echo "` + transferMarker + `sed exited with status $?" >&2; } | "$client" -f -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE; } 2>&1`

	putConfigBackCommand = `client=$(command -v mariadb || command -v mysql); { [ -f /tmp/ccd.sql ] && "$client" -f -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE < /tmp/ccd.sql; } 2>&1`
)

// skippedSQLError is how the client reports a statement it skipped under --force: with the line
// of the dump it was on. A refused connection says ERROR too, but on no line.
var skippedSQLError = regexp.MustCompile(`^ERROR \d+ \([0-9A-Z]+\) at line \d+`)

// skippedErrors is the statements a forced restore went past, in order.
func skippedErrors(said string) []string {
	skipped := []string{}

	for _, line := range strings.Split(said, "\n") {
		if line = strings.TrimSpace(line); skippedSQLError.MatchString(line) {
			skipped = append(skipped, line)
		}
	}

	return skipped
}

// transferDumpAttempts is how many times a dump the database asks to retry is taken.
const transferDumpAttempts = 3

// transferMarker starts the line a part of a pipeline prints on stderr when it fails.
const transferMarker = "hm-transfer-db: "

// dumpPipeline is transfer-db.sh's dump, except that each part says when it failed. pipefail
// would do it under bash, but the remote end is whatever login shell the server gives, which
// may be a plain sh; `part || echo` works in every one of them. gzip, the last part, needs no
// marker: its status is the pipeline's. A ' in a single-quoted value is escaped, where the shell
// let it end the quoting.
func dumpPipeline(host, user, port, passwordFlag, database string) string {
	return fmt.Sprintf(`dump=$(command -v mariadb-dump || command -v mysqldump); `+
		`{ "$dump" --single-transaction --quick --no-tablespaces --skip-comments -h'%s' -u'%s' -P %s %s %s || echo "%sthe dump exited with status $?" >&2; } | `+
		`{ sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' || echo "%ssed exited with status $?" >&2; } | gzip -9`,
		insideQuotes(host), insideQuotes(user), port, passwordFlag, database, transferMarker, transferMarker)
}

// stepFailed reads a pipeline's outcome: a part that said it failed, or a status that is not
// zero. It answers why, in words, and the exit code to stop with — the failed part's own. runner
// is what ran the pipeline when its status is its own and not the pipeline's, as ssh's 255 is.
func stepFailed(said string, status int, runner string) (string, int, bool) {
	for _, line := range strings.Split(said, "\n") {
		why, found := strings.CutPrefix(strings.TrimSpace(line), transferMarker)
		if !found {
			continue
		}

		code := status
		if _, number, ok := strings.Cut(why, "status "); ok {
			if parsed, err := strconv.Atoi(number); err == nil && parsed != 0 {
				code = parsed
			}
		}

		return why, code, true
	}

	switch {
	case status == 0:
		return "", 0, false
	case runner == "ssh" && status == 255:
		return "ssh exited with status 255", status, true
	case runner == "ssh":
		return fmt.Sprintf("the remote command exited with status %d", status), status, true
	}

	return fmt.Sprintf("it exited with status %d", status), status, true
}

// askedToRetry is whether the dump failed with MariaDB's and MySQL's error 1412, "Table
// definition has changed, please retry transaction": a table altered while a consistent
// snapshot was being read, which a live Magento does all the time. Nothing else is retried — a
// refused password or a lost connection is not fixed by asking again.
func askedToRetry(said string) bool {
	return strings.Contains(said, "Error 1412") || strings.Contains(said, "Table definition has changed")
}

// stepTail is the end of what a step said, indented under the failure, without the markers:
// the failure's first line already says what they said.
func stepTail(said string, count int) string {
	lines := []string{}

	for _, line := range strings.Split(said, "\n") {
		line = strings.TrimRight(line, "\r ")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), transferMarker) {
			continue
		}

		lines = append(lines, "  "+line)
	}

	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}

	return strings.Join(lines, "\n")
}

// insideQuotes is a value ready to go between single quotes in a shell command: each quote is
// replaced by one that closes the quoting, an escaped literal quote, and one that opens it again.
func insideQuotes(value string) string {
	return strings.ReplaceAll(value, "'", `'\''`)
}
