package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//
// How transfer-db fails. A user brought staging over SSH, the dump died halfway with MariaDB's
// error 1412, and the transfer copied the truncated file, restored it and ended at the shell
// prompt without a word: these are the cases that must never be silent again.
//

// sshOutcome is what one ssh answers: what the remote side said on stderr, and ssh's status.
type sshOutcome struct {
	said   string
	status int
}

// sshAnswering makes each ssh the command runs answer the next outcome, recording it as dbLab's
// does; once they run out, the last one keeps answering.
func sshAnswering(t *testing.T, ran *[]sshRun, outcomes ...sshOutcome) {
	t.Helper()

	runSSH = func(args []string, output string, complaints io.Writer) (int, error) {
		*ran = append(*ran, sshRun{Args: args, Output: output})

		answer := outcomes[min(len(*ran), len(outcomes))-1]
		io.WriteString(complaints, answer.said) //nolint:errcheck

		return answer.status, nil
	}
}

// failureIn is the envelope a --json failure wrote after whatever was said before it.
func failureIn(t *testing.T, stderr []byte) (kind, message string) {
	t.Helper()

	var failed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	document := stderr
	if at := bytes.IndexByte(document, '{'); at >= 0 {
		document = document[at:]
	}

	if err := json.Unmarshal(document, &failed); err != nil {
		t.Fatalf("stderr is not a failure document: %v\n%s", err, stderr)
	}

	return failed.Error.Type, failed.Error.Message
}

func mustContain(t *testing.T, what, text string, wanted ...string) {
	t.Helper()

	for _, one := range wanted {
		if !strings.Contains(text, one) {
			t.Errorf("%s does not say %q:\n%s", what, one, text)
		}
	}
}

func mustNotContain(t *testing.T, what, text string, unwanted ...string) {
	t.Helper()

	for _, one := range unwanted {
		if strings.Contains(text, one) {
			t.Errorf("%s says %q, which it must not:\n%s", what, one, text)
		}
	}
}

const lostConnection = "mariadb-dump: Got error: 2013: Lost connection to server during query when dumping table `sales_order` at row: 1200\n"

// TestAFailedRemoteDumpStopsBeforeAnythingIsCopied: ssh exits 0 — gzip, the last of the
// pipeline, was fine — but the dump said it failed, so nothing is copied or restored.
func TestAFailedRemoteDumpStopsBeforeAnythingIsCopied(t *testing.T) {
	fake, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: lostConnection + "hm-transfer-db: the dump exited with status 2\n"})

	var stdout, stderr bytes.Buffer

	code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, &stderr, true)
	if code != 2 {
		t.Fatalf("transfer-db with the dump failing = %d, want the dump's 2", code)
	}

	if len(steps(fake)) != 0 {
		t.Fatalf("went on after the dump failed: %v", steps(fake))
	}

	mustNotContain(t, "the output", stderr.String(), "Restoring database dump into localhost")

	kind, message := failureIn(t, stderr.Bytes())
	if kind != "transfer_failed" {
		t.Errorf("failure type = %q, want transfer_failed", kind)
	}

	mustContain(t, "the failure", message,
		"Creating the database dump on ssh.eu-3.magento.cloud failed (the dump exited with status 2)",
		"Got error: 2013: Lost connection to server during query")
	mustNotContain(t, "the failure", message, "hm-transfer-db:")
}

// TestAFailedSSHSaysWhy: what ssh itself said is the reason, and its status is the exit code.
func TestAFailedSSHSaysWhy(t *testing.T) {
	fake, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: "abc@ssh.eu-3.magento.cloud: Permission denied (publickey).\n", status: 255})

	var stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, false); code != 255 {
		t.Fatalf("transfer-db with ssh refused = %d, want 255", code)
	}

	if len(steps(fake)) != 0 {
		t.Fatalf("went on after ssh failed: %v", steps(fake))
	}

	mustContain(t, "stderr", stderr.String(),
		"Creating the database dump on ssh.eu-3.magento.cloud failed (ssh exited with status 255)",
		"Permission denied (publickey).")
}

// TestAFailedLocalDumpStopsBeforeTheRestore: without a tunnel the dump runs in the db container,
// under bash with pipefail, and failing there stops the transfer the same way.
func TestAFailedLocalDumpStopsBeforeTheRestore(t *testing.T) {
	fake, _ := dbLab(t)
	answeringWith(t, "n")

	// Resolve, Describe, the structure-only rules, the dump
	fake.outcomes = []outcome{{}, {}, {}, {status: 2, captured: "mariadb-dump: Got error: 1045: \"Access denied for user 'mysql'@'%'\" when trying to connect\n" +
		"hm-transfer-db: the dump exited with status 2\n"}}

	var stderr bytes.Buffer

	if code := transferDB([]string{"--sql-password=pw"}, io.Discard, &stderr, true); code != 2 {
		t.Fatalf("transfer-db with the local dump failing = %d, want 2", code)
	}

	if got := steps(fake); len(got) != 1 {
		t.Fatalf("went on after the dump failed: %v", got)
	}

	_, message := failureIn(t, stderr.Bytes())
	mustContain(t, "the failure", message,
		"Creating the database dump in the db container failed (the dump exited with status 2)",
		"Access denied for user")
}

const tableChanged = "mariadb-dump: Error 1412: Table definition has changed, please retry transaction when dumping table `sales_data_exporter_orders_v2_sync_batches` at row: 0\n" +
	"hm-transfer-db: the dump exited with status 2\n"

// TestADumpTheDatabaseAsksToRetryIsRetried: 1412 is the server saying "try again", which is what
// happens on a live database while a table is being altered; the second dump goes through.
func TestADumpTheDatabaseAsksToRetryIsRetried(t *testing.T) {
	fake, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: tableChanged}, sshOutcome{})

	var stdout bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db retried = %d, want exitOK", code)
	}

	if len(*ran) != 2 || (*ran)[1].Command() != tunnelledDump {
		t.Fatalf("ssh ran %v, want the same dump twice", *ran)
	}

	mustContain(t, "stdout", stdout.String(), "The table definition changed during the dump, retrying (2/3)...\n")

	if got := steps(fake); len(got) == 0 || got[0] != "CopyIntoService db "+(*ran)[0].Output+" /tmp" {
		t.Fatalf("steps after the retry = %v, want the dump copied once", got)
	}
}

// TestADumpThatKeepsAskingToRetryGivesUp after three attempts, with the database's own words.
func TestADumpThatKeepsAskingToRetryGivesUp(t *testing.T) {
	fake, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: tableChanged})

	var stdout, stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, &stdout, &stderr, false); code != 2 {
		t.Fatalf("transfer-db giving up = %d, want the dump's 2", code)
	}

	if len(*ran) != 3 || len(steps(fake)) != 0 {
		t.Fatalf("ssh ran %d times and then %v, want three dumps and nothing else", len(*ran), steps(fake))
	}

	mustContain(t, "stdout", stdout.String(), "retrying (2/3)...\n", "retrying (3/3)...\n")
	mustContain(t, "stderr", stderr.String(),
		"Creating the database dump on ssh.eu-3.magento.cloud failed after 3 attempts (the dump exited with status 2)",
		"Error 1412: Table definition has changed, please retry transaction")
}

// TestOtherDumpErrorsAreNotRetried: a lost connection or a refused password will not fix itself.
func TestOtherDumpErrorsAreNotRetried(t *testing.T) {
	_, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: lostConnection + "hm-transfer-db: the dump exited with status 2\n"}, sshOutcome{})

	var stdout bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc"}, &stdout, io.Discard, false); code != 2 {
		t.Fatalf("transfer-db = %d, want 2", code)
	}

	if len(*ran) != 1 {
		t.Fatalf("ssh ran %d times, want one: only 1412 is retried", len(*ran))
	}

	mustNotContain(t, "stdout", stdout.String(), "retrying")
}

// TestTheLocalDumpIsRetriedToo: the same database error without a tunnel, re-running only the
// dump in the db container.
func TestTheLocalDumpIsRetriedToo(t *testing.T) {
	fake, _ := dbLab(t)
	answeringWith(t, "n")

	// Resolve, Describe, the structure-only rules, the first dump
	fake.outcomes = []outcome{{}, {}, {}, {captured: tableChanged}}

	if code := transferDB([]string{"--sql-password=pw"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	got := steps(fake)
	if len(got) < 3 || got[0] != got[1] || !strings.Contains(got[0], "/tmp/db.sql.gz") ||
		strings.Contains(got[2], "/tmp/db.sql.gz") {
		t.Fatalf("steps = %v, want the dump twice and then the rest", got)
	}
}

// TestEveryFailingStepSaysWhichAndWhy: each step after the dump stops the transfer with its
// status, its name, and the end of what it said — where it used to stop with the status alone.
func TestEveryFailingStepSaysWhichAndWhy(t *testing.T) {
	cases := []struct {
		name string

		// at is the call that fails: Resolve, Describe, the structure-only rules, Copy, keep the
		// config, restore, put the config back, Clear, Anonymise and the three bin/magento follow
		at       int
		outcome  outcome
		code     int
		headline string
		said     string
	}{
		{
			name: "copying the dump", at: 3, outcome: outcome{err: errors.New("no space left on device")},
			code: exitDocker, headline: "Copying the dump into the db container failed: no space left on device",
		},
		{
			name: "saving core_config_data", at: 4,
			outcome: outcome{status: 2, captured: "mariadb-dump: Got error: 1045: \"Access denied for user 'magento'@'localhost'\"\n"},
			code:    2, headline: "Saving core_config_data, admin_user and tfa_user_config failed (it exited with status 2)",
			said: "Access denied for user 'magento'@'localhost'",
		},
		{
			name: "restoring a truncated dump", at: 5,
			outcome: outcome{status: 1, captured: "gzip: stdin: unexpected end of file\nhm-transfer-db: zcat exited with status 1\n"},
			code:    1, headline: "Restoring the dump into the local database failed (zcat exited with status 1)",
			said: "gzip: stdin: unexpected end of file",
		},
		{
			name: "restoring with no server to restore into", at: 5,
			outcome: outcome{status: 1, captured: "ERROR 2002 (HY000): Can't connect to local server through socket '/run/mysqld/mysqld.sock' (2)\n"},
			code:    1, headline: "Restoring the dump into the local database failed (it exited with status 1)",
			said: "ERROR 2002 (HY000): Can't connect to local server",
		},
		{
			name: "putting core_config_data back", at: 6,
			outcome: outcome{status: 1, captured: "ERROR 1062 (23000) at line 30: Duplicate entry '1' for key 'PRIMARY'\n"},
			code:    1, headline: "Putting core_config_data, admin_user and tfa_user_config back failed (it exited with status 1)",
			said: "Duplicate entry '1' for key 'PRIMARY'",
		},
		{
			name: "clearing the anonymisation record", at: 7, outcome: outcome{err: errors.New("read-only file system")},
			code: exitDocker, headline: "Recording that the database is no longer anonymised failed: read-only file system",
		},
		{
			name: "reindexing", at: 10, outcome: outcome{status: 1},
			code: 1, headline: "Running bin/magento indexer:reindex failed (it exited with status 1; its output is above)",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake, _ := dbLab(t)
			fake.outcomes = make([]outcome, tt.at+1)
			fake.outcomes[tt.at] = tt.outcome

			var stderr bytes.Buffer

			if code := transferDB([]string{"--ssh-user=abc", "--anonymise=true"}, io.Discard, &stderr, true); code != tt.code {
				t.Fatalf("transfer-db = %d, want %d\n%s", code, tt.code, stderr.String())
			}

			if got := steps(fake); len(got) != tt.at-2 {
				t.Fatalf("went on after the failing step: %v", got)
			}

			_, message := failureIn(t, stderr.Bytes())
			mustContain(t, "the failure", message, tt.headline, tt.said)
		})
	}
}

// TestSavingCoreConfigDataNoLongerHidesItsErrors: its stderr went to /dev/null.
func TestSavingCoreConfigDataNoLongerHidesItsErrors(t *testing.T) {
	fake, _ := dbLab(t)

	transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false)

	for _, step := range steps(fake) {
		if strings.Contains(step, "/dev/null") {
			t.Fatalf("a step still throws its errors away: %s", step)
		}
	}
}

// TestErrorsTheForcedRestoreSkippedAreCounted: the restore keeps --force — a dump from another
// server can carry a statement this one refuses, and stopping there would leave half a database
// — but what it skipped is said at the end instead of vanishing.
func TestErrorsTheForcedRestoreSkippedAreCounted(t *testing.T) {
	fake, _ := dbLab(t)

	// Resolve, Describe, the structure-only rules, Copy, keep the config, restore
	fake.outcomes = []outcome{{}, {}, {}, {}, {}, {status: 1, captured: "ERROR 1146 (42S02) at line 12: Table 'main.gone' doesn't exist\n" +
		"ERROR 1064 (42000) at line 40: You have an error in your SQL syntax\n"}}

	var stdout, stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc", "--anonymise=true"}, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("transfer-db with skipped errors = %d, want exitOK\n%s", code, stderr.String())
	}

	if !contains(steps(fake), "Exec phpfpm bin/magento | cache:flush") {
		t.Fatalf("stopped at the skipped errors: %v", steps(fake))
	}

	mustContain(t, "stderr", stderr.String(),
		"The restore skipped 2 SQL errors; the first: ERROR 1146 (42S02) at line 12: Table 'main.gone' doesn't exist")
	mustContain(t, "stdout", stdout.String(), " All done!\n")
}

// transferLogPath is the one log the transfer wrote: beside the rest of what hm keeps on this
// machine, which a test's HM_STATE_DIR moves into its own directory.
func transferLogPath(t *testing.T) string {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(filepath.Dir(os.Getenv("HM_STATE_DIR")), "logs", "transfer-db-shop-*.log"))
	if err != nil || len(found) != 1 {
		t.Fatalf("transfer logs = %v (%v), want exactly one", found, err)
	}

	return found[0]
}

func transferLogText(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile(transferLogPath(t))
	if err != nil {
		t.Fatal(err)
	}

	return string(contents)
}

// TestTheTransferIsLogged: every step, its command and what it said, for whoever has to find out
// afterwards what happened.
func TestTheTransferIsLogged(t *testing.T) {
	fake, _ := dbLab(t)

	// Resolve, Describe, the structure-only rules, Copy, keep the config, restore
	fake.outcomes = []outcome{{}, {}, {}, {}, {captured: "mariadb-dump: [Warning] Using a password on the command line interface can be insecure.\n"},
		{status: 1, captured: "ERROR 1146 (42S02) at line 12: Table 'main.gone' doesn't exist\n"}}

	if code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw", "--anonymise=false"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	log := transferLogText(t)
	mustContain(t, "the log", log,
		"Creating the database dump on ssh.eu-3.magento.cloud",
		"abc@ssh.eu-3.magento.cloud",
		"-p'***' main",
		"Copying the dump into the db container",
		"Saving core_config_data, admin_user and tfa_user_config",
		"Using a password on the command line interface can be insecure.",
		"Restoring the dump into the local database",
		"ERROR 1146 (42S02) at line 12: Table 'main.gone' doesn't exist",
		"Running bin/magento cache:flush",
		" All done!")
	mustNotContain(t, "the log", log, "-p'pw'")
}

// TestARestoreThatLostTheServerFails: under --force the client goes on past a statement the
// server refuses, and it reports a lost connection the same way, "at line N" — but a server that
// went away mid-restore leaves half a database, which is a failure and not a skipped statement.
func TestARestoreThatLostTheServerFails(t *testing.T) {
	cases := []struct {
		name, captured, line string
		status               int
	}{
		{
			name:     "lost connection after a skipped statement",
			captured: "ERROR 1146 (42S02) at line 12: Table 'main.gone' doesn't exist\nERROR 2013 (HY000) at line 3456: Lost connection to server during query\n",
			line:     "ERROR 2013 (HY000) at line 3456: Lost connection to server during query", status: 1,
		},
		{
			name:     "server gone away, again on every line after",
			captured: "ERROR 2006 (HY000) at line 3457: Server has gone away\nERROR 2006 (HY000) at line 3458: Server has gone away\n",
			line:     "ERROR 2006 (HY000) at line 3457: Server has gone away", status: 1,
		},
		{
			name:     "even with a status of zero",
			captured: "ERROR 2013 (HY000) at line 9: Lost connection to MySQL server during query\n",
			line:     "ERROR 2013 (HY000) at line 9: Lost connection to MySQL server during query",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake, _ := dbLab(t)

			// Resolve, Describe, the structure-only rules, Copy, keep the config, restore
			fake.outcomes = []outcome{{}, {}, {}, {}, {}, {status: tt.status, captured: tt.captured}}

			var stderr bytes.Buffer

			code := transferDB([]string{"--ssh-user=abc", "--anonymise=false"}, io.Discard, &stderr, true)
			if want := max(tt.status, exitError); code != want {
				t.Fatalf("transfer-db = %d, want %d\n%s", code, want, stderr.String())
			}

			if contains(steps(fake), "ClearAnonymisation") {
				t.Fatalf("went on after the server was lost: %v", steps(fake))
			}

			_, message := failureIn(t, stderr.Bytes())
			mustContain(t, "the failure", message,
				"Restoring the dump into the local database failed (the database server was lost: "+tt.line+")")
			mustNotContain(t, "the failure", message, "skipped")
		})
	}
}

// TestEveryFailureEndsWithTheLog: the message is the start, the log is the rest of the story.
func TestEveryFailureEndsWithTheLog(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		fake, _ := dbLab(t)
		fake.outcomes = []outcome{{}, {}, {}, {}, {}, {status: 1, captured: "gzip: stdin: unexpected end of file\nhm-transfer-db: zcat exited with status 1\n"}}

		var stderr bytes.Buffer

		transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, true)

		_, message := failureIn(t, stderr.Bytes())
		if want := "\nThe whole transfer is logged in " + transferLogPath(t); !strings.HasSuffix(message, want) {
			t.Fatalf("failure = %q, want it to end with %q", message, want)
		}

		mustContain(t, "the log", transferLogText(t), "Restoring the dump into the local database failed (zcat exited with status 1)")
	})

	t.Run("text", func(t *testing.T) {
		_, ran := dbLab(t)
		sshAnswering(t, ran, sshOutcome{said: "Connection closed by 1.2.3.4 port 22\n", status: 255})

		var stderr bytes.Buffer

		transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, false)

		mustContain(t, "stderr", stderr.String(), "Connection closed by 1.2.3.4 port 22\nThe whole transfer is logged in "+transferLogPath(t)+"\n")
	})
}

// TestTheLogKeepsEveryAttempt: a retried dump leaves the reason of the attempt that failed.
func TestTheLogKeepsEveryAttempt(t *testing.T) {
	_, ran := dbLab(t)
	sshAnswering(t, ran, sshOutcome{said: tableChanged}, sshOutcome{})

	transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false)

	mustContain(t, "the log", transferLogText(t), "(attempt 1/3)", "Error 1412: Table definition has changed",
		"retrying (2/3)...", "(attempt 2/3)")
}

// TestThePasswordIsNeverShownOrLogged, not even when the remote side repeats it.
func TestThePasswordIsNeverShownOrLogged(t *testing.T) {
	_, ran := dbLab(t)

	const password = "S3cr3t'pw"

	sshAnswering(t, ran, sshOutcome{said: "mariadb-dump: cannot connect with -p'S3cr3t'\\''pw' or S3cr3t'pw\nhm-transfer-db: the dump exited with status 2\n"})

	var stdout, stderr bytes.Buffer

	transferDB([]string{"--ssh-user=abc", "--sql-password=" + password}, &stdout, &stderr, false)

	for what, text := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String(), "the log": transferLogText(t)} {
		mustNotContain(t, what, text, "S3cr3t")
	}

	mustContain(t, "the log", transferLogText(t), "-p'***' main")
}

// TestALogThatCannotBeWrittenDoesNotStopTheTransfer: it says so, and goes on.
func TestALogThatCannotBeWrittenDoesNotStopTheTransfer(t *testing.T) {
	dbLab(t)

	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// The logs directory would be inside a file
	t.Setenv("HM_STATE_DIR", filepath.Join(blocked, "logs", "state"))

	var stdout, stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc", "--anonymise=false"}, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("transfer-db without a log = %d, want exitOK", code)
	}

	mustContain(t, "stderr", stderr.String(), "The transfer is not being logged:")
	mustContain(t, "stdout", stdout.String(), " All done!\n")
	mustNotContain(t, "stdout", stdout.String(), "logged in")
}
