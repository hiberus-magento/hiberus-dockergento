package cli

import (
	"bytes"
	"encoding/json"
	"io"
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

	// Resolve, Describe, the dump
	fake.outcomes = []outcome{{}, {}, {status: 2, captured: "mariadb-dump: Got error: 1045: \"Access denied for user 'mysql'@'%'\" when trying to connect\n" +
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

	if len(*ran) != 2 || (*ran)[1].Args[len((*ran)[1].Args)-1] != tunnelledDump {
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

	// Resolve, Describe, the first dump
	fake.outcomes = []outcome{{}, {}, {captured: tableChanged}}

	if code := transferDB([]string{"--sql-password=pw"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	got := steps(fake)
	if len(got) < 3 || got[0] != got[1] || !strings.Contains(got[0], "/tmp/db.sql.gz") ||
		strings.Contains(got[2], "/tmp/db.sql.gz") {
		t.Fatalf("steps = %v, want the dump twice and then the rest", got)
	}
}
