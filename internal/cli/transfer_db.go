package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// runSSH runs ssh with its output into a file and the rest attached to this terminal, so that SSH
// can ask for a passphrase. It answers ssh's own exit status. A package variable so that no test
// ever connects anywhere.
var runSSH = func(args []string, output string) (int, error) {
	file, err := os.Create(output) //nolint:gosec
	if err != nil {
		return 0, err
	}
	defer file.Close()

	command := exec.Command("ssh", args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, file, os.Stderr

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
		passwordFlag = "-p'" + sqlPassword + "'"
	}

	exclude := yes(question("Do you want to exclude 'core_config_data' table? [Y/n]:", ""))

	say(fmt.Sprintf("You are going to transfer database from [%s:[%s:%s]] to [LOCALHOST].\n", sshHost, sqlHost, sqlPort))
	question("Press any key continue...", "")

	if sqlHost == "" || sqlPort == "" || sqlUser == "" || sqlDB == "" {
		return failure(stderr, jsonOutput, "transfer-db", exitUsage, "input_required",
			"Missing connection data for the remote database", binaryName()+" transfer-db --help")
	}

	say("Creating database dump from origin server...\n")

	// What the shell ran, byte for byte, the spacing of its line continuations included
	dump := fmt.Sprintf(`dump=$(command -v mariadb-dump || command -v mysqldump); "$dump" --single-transaction --quick --no-tablespaces --skip-comments -h'%s' -u'%s' -P %s %s %s`,
		sqlHost, sqlUser, sqlPort, passwordFlag, sqlDB)
	definers := `sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/'`

	// inside runs one step in a container, and stops with its status as set -e did
	inside := func(service string, command ...string) int {
		status, err := engine.Exec(project.Root, service, command, terminalOptions(""))
		if status != 0 {
			return status
		}

		return report(stderr, jsonOutput, "transfer-db", err)
	}

	if sshHost == "" {
		if status := inside("db", "bash", "-c", dump+" | "+definers+" | gzip -9 > /tmp/db.sql.gz"); status != 0 {
			return status
		}
	} else {
		local := filepath.Join(here(), "db.sql.gz")

		status, err := runSSH([]string{sshUser + "@" + sshHost,
			dump + "         | " + definers + "         | gzip -9"}, local)
		if err != nil {
			return failure(stderr, jsonOutput, "transfer-db", exitError, "transfer_failed",
				"The database dump could not be taken over SSH: "+err.Error(), "")
		}

		if status != 0 {
			return status
		}

		if err := engine.CopyIntoService(project.Root, "db", local, "/tmp"); err != nil {
			return report(stderr, jsonOutput, "transfer-db", err)
		}
	}

	say("Restoring database dump into localhost...\n")

	if exclude {
		if status := inside("db", "bash", "-c", `dump=$(command -v mariadb-dump || command -v mysqldump); "$dump" -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE core_config_data admin_user tfa_user_config > /tmp/ccd.sql 2> /dev/null`); status != 0 {
			return status
		}
	}

	if status := inside("db", "bash", "-c", `client=$(command -v mariadb || command -v mysql); zcat /tmp/db.sql.gz | sed '/sandbox mode/d' | "$client" -f -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE`); status != 0 {
		return status
	}

	if exclude {
		if status := inside("db", "bash", "-c", `client=$(command -v mariadb || command -v mysql); [ -f /tmp/ccd.sql ] && "$client" -f -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE < /tmp/ccd.sql`); status != 0 {
			return status
		}
	}

	// A database from another environment has just replaced this one: whatever was true about
	// the old contents is not true about these
	if err := engine.ClearAnonymisation(project.Root); err != nil {
		return report(stderr, jsonOutput, "transfer-db", err)
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

		if status := inside(phpService, "bin/magento", step.command); status != 0 {
			return status
		}
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
