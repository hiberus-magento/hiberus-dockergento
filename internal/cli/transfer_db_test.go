package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// transfer-db brings a remote database into this project's: a dump taken over SSH (or straight
// from the db container when there is no tunnel), restored, optionally anonymised, and followed
// by Magento's own config import, reindex and cache flush — console/commands/transfer-db.sh,
// questions, defaults and commands included. Never a real ssh here and never a daemon: the ssh
// runner and the questions are replaced, and what they were asked is what is checked.
//

// The commands transfer-db runs. They are transfer-db.sh's, except that each part of a pipeline
// says on stderr when it failed — the remote login shell is not guaranteed to be bash, so
// pipefail cannot be counted on there, and without it a failed dump left a truncated file that
// was copied and restored as if nothing had happened — and that the steps in the db container
// keep what they say, which a failure quotes.
const (
	tunnelledDump = `dump=$(command -v mariadb-dump || command -v mysqldump); { "$dump" --single-transaction --quick --no-tablespaces --skip-comments -h'database.internal' -u'mysql' -P 3306 -p'pw' main || echo "hm-transfer-db: the dump exited with status $?" >&2; } | { sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' || echo "hm-transfer-db: sed exited with status $?" >&2; } | gzip -9`
	directDump    = `{ ` + tunnelledDump + ` > /tmp/db.sql.gz; } 2>&1`
	keepConfig    = keepConfigCommand
	restoreDump   = `client=$(command -v mariadb || command -v mysql); { { zcat /tmp/db.sql.gz || echo "hm-transfer-db: zcat exited with status $?" >&2; } | { sed '/sandbox mode/d' || echo "hm-transfer-db: sed exited with status $?" >&2; } | "$client" -f -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DATABASE; } 2>&1`
	putConfigBack = putConfigBackCommand
)

// sshRun is one ssh the command would have run: its arguments and where its output went.
type sshRun struct {
	Args   []string
	Output string
}

// Destination and Command are ssh's last two arguments: user@host and what runs there. The
// options come before them.
func (r sshRun) Destination() string { return r.Args[len(r.Args)-2] }
func (r sshRun) Command() string     { return r.Args[len(r.Args)-1] }

// dbLab is a project whose phpfpm and db are running, with a recording ssh.
func dbLab(t *testing.T) (*fakeEngine, *[]sshRun) {
	t.Helper()

	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.project = core.Project{Name: "shop", Root: here()}
	fake.description.Project.Name = "shop"
	fake.description.Services = []core.DescribedService{
		{Name: "db", State: "running"},
		{Name: "phpfpm", State: "running"},
	}

	ran := &[]sshRun{}

	previous := runSSH
	t.Cleanup(func() { runSSH = previous })

	runSSH = func(args []string, output string, _ io.Writer) (int, error) {
		*ran = append(*ran, sshRun{Args: args, Output: output})

		return 0, nil
	}

	return fake, ran
}

// answeringWith makes the session interactive and answers its questions in order, recording
// each question and the default it offered.
func answeringWith(t *testing.T, answers ...string) *[]string {
	t.Helper()

	t.Setenv("HM_NON_INTERACTIVE", "")

	asked := &[]string{}

	previousAsk, previousSecret := askTransfer, readSecret
	t.Cleanup(func() { askTransfer, readSecret = previousAsk, previousSecret })

	next := func(question string) string {
		*asked = append(*asked, question)

		if len(answers) == 0 {
			return ""
		}

		answer := answers[0]
		answers = answers[1:]

		return answer
	}

	askTransfer = func(text, suggestion string) (string, error) {
		answer := next(text + " {" + suggestion + "}")
		if answer == "" {
			return suggestion, nil
		}

		return answer, nil
	}

	readSecret = func(text string) (string, error) {
		return next(text + " {secret}"), nil
	}

	return asked
}

// steps is what the command asked of the engine after resolving and describing the project and
// reading its structure-only rules.
func steps(fake *fakeEngine) []string {
	out := []string{}

	for _, one := range fake.calls {
		switch one.Method {
		case "Resolve", "Describe", "Property":
			continue
		case "Exec", "Capture":
			out = append(out, one.Method+" "+one.Service+" "+strings.Join(one.Command, " | "))
		case "CopyIntoService":
			out = append(out, "CopyIntoService "+one.Service+" "+strings.Join(one.Paths, " ")+" "+one.Path)
		default:
			out = append(out, one.Method)
		}
	}

	return out
}

func magentoSteps() []string {
	return []string{
		"Exec phpfpm bin/magento | app:config:import",
		"Exec phpfpm bin/magento | indexer:reindex",
		"Exec phpfpm bin/magento | cache:flush",
	}
}

func TestTransferDBDumpsOverSSHAndRestoresIntoTheDbContainer(t *testing.T) {
	fake, ran := dbLab(t)

	code := transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, io.Discard, io.Discard, false)
	if code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	wantSSH := []sshRun{{Args: append(sshOptions(), "abc@ssh.eu-3.magento.cloud", tunnelledDump),
		Output: filepath.Join(here(), "db.sql.gz")}}
	if diff := cmp.Diff(wantSSH, *ran); diff != "" {
		t.Fatalf("ssh (-want +got):\n%s", diff)
	}

	want := append([]string{
		"CopyIntoService db " + filepath.Join(here(), "db.sql.gz") + " /tmp",
		"Capture db bash | -o | pipefail | -c | " + keepConfig,
		"Capture db bash | -o | pipefail | -c | " + restoreDump,
		"Capture db bash | -o | pipefail | -c | " + putConfigBack,
		"ClearAnonymisation",
		"Anonymise",
	}, magentoSteps()...)
	if diff := cmp.Diff(want, steps(fake)); diff != "" {
		t.Fatalf("steps (-want +got):\n%s", diff)
	}
}

// TestEveryOptionReachesTheCommandsItBuilds: the eight options of the shell, each in its place.
func TestEveryOptionReachesTheCommandsItBuilds(t *testing.T) {
	_, ran := dbLab(t)

	transferDB([]string{"--ssh-host=ssh.example.cloud", "--ssh-user=abc", "--sql-host=db.remote",
		"--sql-port=3307", "--sql-user=shop", "--sql-db=live", "--sql-password=secret",
		"--anonymise=false"}, io.Discard, io.Discard, false)

	want := strings.NewReplacer("'database.internal'", "'db.remote'", "'mysql'", "'shop'", "3306", "3307",
		"'pw' main", "'secret' live").Replace(tunnelledDump)
	if len(*ran) != 1 || (*ran)[0].Destination() != "abc@ssh.example.cloud" || (*ran)[0].Command() != want {
		t.Fatalf("ssh ran %v, want abc@ssh.example.cloud with %s", *ran, want)
	}
}

// TestWithoutAPasswordThereIsNoPasswordFlag: the shell left an empty word, and so does this.
func TestWithoutAPasswordThereIsNoPasswordFlag(t *testing.T) {
	_, ran := dbLab(t)

	transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false)

	if len(*ran) != 1 || !strings.Contains((*ran)[0].Command(), "-P 3306  main ") ||
		strings.Contains((*ran)[0].Command(), "-p'") {
		t.Fatalf("ssh ran %v, want no -p", *ran)
	}
}

func TestWithoutATunnelTheDumpIsTakenFromTheDbContainer(t *testing.T) {
	fake, ran := dbLab(t)
	answeringWith(t, "n")

	if code := transferDB([]string{"--sql-password=pw"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}

	if len(*ran) != 0 {
		t.Fatalf("ssh ran without a tunnel: %v", *ran)
	}

	want := append([]string{
		"Capture db bash | -o | pipefail | -c | " + directDump,
		"Capture db bash | -o | pipefail | -c | " + keepConfig,
		"Capture db bash | -o | pipefail | -c | " + restoreDump,
		"Capture db bash | -o | pipefail | -c | " + putConfigBack,
		"ClearAnonymisation",
		"Anonymise",
	}, magentoSteps()...)
	if diff := cmp.Diff(want, steps(fake)); diff != "" {
		t.Fatalf("steps (-want +got):\n%s", diff)
	}
}

// TestTheQuestionsAreTheShellsInItsOrder: same questions, same defaults, the password last of the
// connection and never shown.
func TestTheQuestionsAreTheShellsInItsOrder(t *testing.T) {
	dbLab(t)
	asked := answeringWith(t)

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, io.Discard, io.Discard, false)

	want := []string{
		"Do you need to use SSH tunneling? [Y/n]: {}",
		"SSH Host {ssh.eu-3.magento.cloud}",
		"SSH User {abc}",
		"Database Host {database.internal}",
		"Database Port {3306}",
		"Database User {mysql}",
		"Database DB Name {main}",
		"Database Password {secret}",
		"Do you want to exclude 'core_config_data' table? [Y/n]: {}",
		"Press any key continue... {}",
		"Do you want to anonymise the database? [Y/n]: {}",
		"Do you want to import app config (app:config:import)? [Y/n]: {}",
		"Do you want to reindex Magento? [Y/n]: {}",
		"Do you want to clear Magento cache? [Y/n]: {}",
	}
	if diff := cmp.Diff(want, *asked); diff != "" {
		t.Fatalf("questions (-want +got):\n%s", diff)
	}
}

// TestTheAnswersReplaceTheDefaults: what is typed wins over the option, and an empty password
// answer keeps the one passed.
func TestTheAnswersReplaceTheDefaults(t *testing.T) {
	_, ran := dbLab(t)
	answeringWith(t, "y", "ssh.other", "me", "db.other", "3310", "reader", "shop")

	transferDB([]string{"--sql-password=pw"}, io.Discard, io.Discard, false)

	want := strings.NewReplacer("'database.internal'", "'db.other'", "'mysql'", "'reader'", "3306", "3310",
		"'pw' main", "'pw' shop").Replace(tunnelledDump)
	if len(*ran) != 1 || (*ran)[0].Destination() != "me@ssh.other" || (*ran)[0].Command() != want {
		t.Fatalf("ssh ran %v", *ran)
	}
}

// TestNonInteractiveAsksNothing: every answer falls back to the options and the defaults.
func TestNonInteractiveAsksNothing(t *testing.T) {
	dbLab(t)

	previousAsk, previousSecret := askTransfer, readSecret
	t.Cleanup(func() { askTransfer, readSecret = previousAsk, previousSecret })

	askTransfer = func(text, _ string) (string, error) {
		t.Errorf("asked %q in non-interactive mode", text)

		return "", nil
	}
	readSecret = func(text string) (string, error) {
		t.Errorf("asked %q in non-interactive mode", text)

		return "", nil
	}

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db = %d, want exitOK", code)
	}
}

func TestNotExcludingCoreConfigDataRestoresTheDumpAlone(t *testing.T) {
	fake, _ := dbLab(t)
	answeringWith(t, "", "", "", "", "", "", "", "", "n")

	transferDB([]string{"--ssh-user=abc", "--sql-password=pw"}, io.Discard, io.Discard, false)

	for _, step := range steps(fake) {
		if strings.Contains(step, "ccd.sql") {
			t.Fatalf("core_config_data was kept although the answer was no: %s", step)
		}
	}
}

func TestDecliningTheFollowUpsSkipsThem(t *testing.T) {
	fake, _ := dbLab(t)
	answeringWith(t, "", "", "", "", "", "", "", "", "", "", "n", "n", "n", "n")

	var stdout bytes.Buffer
	transferDB([]string{"--ssh-user=abc"}, &stdout, io.Discard, false)

	for _, step := range steps(fake) {
		if step == "Anonymise" || strings.HasPrefix(step, "Exec phpfpm") {
			t.Fatalf("%s ran although the answer was no", step)
		}
	}

	if !strings.Contains(stdout.String(), "Skipping database anonymisation...\n") {
		t.Errorf("stdout = %q, want the skip said", stdout.String())
	}
}

func TestTheAnonymiseOptionDecidesWithoutAsking(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		fake, _ := dbLab(t)
		asked := answeringWith(t)

		transferDB([]string{"--ssh-user=abc", "--anonymise=true"}, io.Discard, io.Discard, false)

		if !contains(steps(fake), "Anonymise") {
			t.Fatalf("not anonymised: %v", steps(fake))
		}

		if contains(*asked, "Do you want to anonymise the database? [Y/n]: {}") {
			t.Fatal("asked whether to anonymise although --anonymise=true said so")
		}
	})

	t.Run("false", func(t *testing.T) {
		fake, _ := dbLab(t)
		var stdout bytes.Buffer

		transferDB([]string{"--ssh-user=abc", "--anonymise=false"}, &stdout, io.Discard, false)

		if contains(steps(fake), "Anonymise") {
			t.Fatalf("anonymised although --anonymise=false: %v", steps(fake))
		}

		if !strings.Contains(stdout.String(), "Skipping database anonymisation...\n") {
			t.Errorf("stdout = %q, want the skip said", stdout.String())
		}
	})
}

// TestTheMessagesAreTheShells: the same lines, on stdout, and on stderr under --json, where stdout
// is nobody's but the containers' — and, after them, where the transfer was logged.
func TestTheMessagesAreTheShells(t *testing.T) {
	want := "Database transfer assistant: \n" +
		"You are going to transfer database from [ssh.eu-3.magento.cloud:[database.internal:3306]] to [LOCALHOST].\n" +
		"Creating database dump from origin server...\n" +
		"Restoring database dump into localhost...\n" +
		" All done!\n"

	t.Run("text", func(t *testing.T) {
		dbLab(t)
		var stdout, stderr bytes.Buffer

		transferDB([]string{"--ssh-user=abc", "--anonymise=true"}, &stdout, &stderr, false)

		if want := want + "The transfer was logged in " + transferLogPath(t) + "\n"; stdout.String() != want {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	})

	t.Run("json", func(t *testing.T) {
		dbLab(t)
		var stdout, stderr bytes.Buffer

		transferDB([]string{"--ssh-user=abc", "--anonymise=true"}, &stdout, &stderr, true)

		if want := want + "The transfer was logged in " + transferLogPath(t) + "\n"; stdout.Len() != 0 || stderr.String() != want {
			t.Fatalf("stdout = %q, stderr = %q, want everything on stderr", stdout.String(), stderr.String())
		}
	})
}

func TestTransferDBRefusesMissingConnectionData(t *testing.T) {
	fake, ran := dbLab(t)
	var stderr bytes.Buffer

	code := transferDB([]string{"--ssh-user=abc", "--sql-host="}, io.Discard, &stderr, true)
	if code != exitUsage {
		t.Fatalf("transfer-db with no database host = %d, want exitUsage", code)
	}

	assertFailure(t, stderr.Bytes(), "input_required", "Missing connection data for the remote database",
		"hm transfer-db --help")

	if len(*ran) != 0 || len(steps(fake)) != 0 {
		t.Fatalf("ran %v and %v after refusing", *ran, steps(fake))
	}
}

func TestTransferDBNeedsPhpfpmAndDbRunning(t *testing.T) {
	for _, service := range []string{"phpfpm", "db"} {
		t.Run(service, func(t *testing.T) {
			fake, ran := dbLab(t)
			for at := range fake.description.Services {
				if fake.description.Services[at].Name == service {
					fake.description.Services[at].State = "exited"
				}
			}

			var stdout, stderr bytes.Buffer

			if code := transferDB([]string{"--ssh-user=abc"}, &stdout, &stderr, true); code != exitService {
				t.Fatalf("transfer-db with %s stopped = %d, want exitService", service, code)
			}

			assertFailure(t, stderr.Bytes(), "service_not_running",
				"Service '"+service+"' is not running", "hm start "+service)

			if len(*ran) != 0 || len(steps(fake)) != 0 || stdout.Len() != 0 {
				t.Fatalf("ran %v and %v after refusing", *ran, steps(fake))
			}
		})
	}
}

func TestTransferDBOutsideAProjectIsRefused(t *testing.T) {
	fake, ran := dbLab(t)
	fake.project = core.Project{}
	var stderr bytes.Buffer

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, &stderr, true); code != exitProject {
		t.Fatalf("transfer-db outside a project = %d, want exitProject", code)
	}

	assertFailure(t, stderr.Bytes(), "project_not_configured",
		"This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid",
		"hm setup")

	if len(*ran) != 0 {
		t.Fatalf("ssh ran outside a project: %v", *ran)
	}
}

// TestAFailedSSHStopsWithItsStatus: under set -e the shell stopped at the failing ssh, with its
// code, before restoring anything.
func TestAFailedSSHStopsWithItsStatus(t *testing.T) {
	fake, _ := dbLab(t)
	runSSH = func([]string, string, io.Writer) (int, error) { return 255, nil }

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false); code != 255 {
		t.Fatalf("transfer-db with ssh failing = %d, want 255", code)
	}

	if len(steps(fake)) != 0 {
		t.Fatalf("went on after ssh failed: %v", steps(fake))
	}
}

// TestAFailedStepStopsWithItsStatus: the restore failing is the end, and the record of the data
// is not touched.
func TestAFailedStepStopsWithItsStatus(t *testing.T) {
	fake, _ := dbLab(t)
	// Resolve, Describe, the structure-only rules, CopyIntoService, keep the config, restore
	fake.outcomes = []outcome{{}, {}, {}, {}, {}, {status: 1}}

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false); code != 1 {
		t.Fatalf("transfer-db with the restore failing = %d, want 1", code)
	}

	if contains(steps(fake), "ClearAnonymisation") {
		t.Fatalf("went on after the restore failed: %v", steps(fake))
	}
}

// TestAFailedAnonymisationDoesNotStopTheTransfer: `masquerade_run && hm_anonymisation_record`
// never stopped the shell, and the rest of the steps still ran.
func TestAFailedAnonymisationDoesNotStopTheTransfer(t *testing.T) {
	fake, _ := dbLab(t)
	// Resolve, Describe, the structure-only rules, Copy, three execs, Clear, Anonymise
	fake.outcomes = []outcome{{}, {}, {}, {}, {}, {}, {}, {}, {err: errors.New("masquerade failed")}}

	if code := transferDB([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db with the anonymiser failing = %d, want exitOK", code)
	}

	if !contains(steps(fake), "Exec phpfpm bin/magento | cache:flush") {
		t.Fatalf("stopped after the anonymiser failed: %v", steps(fake))
	}
}

// TestTransferDBIgnoresAnOptionItDoesNotKnow, as the shell's catch-all case did.
func TestTransferDBIgnoresAnOptionItDoesNotKnow(t *testing.T) {
	_, ran := dbLab(t)

	if code := transferDB([]string{"--ssh-user=abc", "--ssh-port=22", "extra"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-db with an unknown option = %d, want exitOK", code)
	}

	if len(*ran) != 1 {
		t.Fatalf("ssh ran %v", *ran)
	}
}

// TestTheRouterAnswersForTransferDB: it reaches its handler, not the shell.
func TestTheRouterAnswersForTransferDB(t *testing.T) {
	_, ran := dbLab(t)

	Run([]string{"transfer-db", "--ssh-user=abc"}, io.Discard, io.Discard)

	if len(*ran) != 1 {
		t.Fatalf("transfer-db did not reach its handler: ssh ran %v", *ran)
	}
}

func assertFailure(t *testing.T, document []byte, kind, message, hint string) {
	t.Helper()

	var failed struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}

	// The messages said before the refusal come first, on the same stream, as in the shell
	if at := bytes.IndexByte(document, '{'); at > 0 {
		document = document[at:]
	}

	if err := json.Unmarshal(document, &failed); err != nil {
		t.Fatalf("stderr is not a document: %v\n%s", err, document)
	}

	if failed.Error.Type != kind || failed.Error.Message != message || failed.Error.Hint != hint {
		t.Fatalf("failure = %+v, want %s / %s / %s", failed.Error, kind, message, hint)
	}
}

// TestAQuoteInAConnectionValueStaysInsideItsWord: host, user and password go single-quoted into a
// shell command, so a ' in one of them is escaped rather than ending the quoting — a password
// with a quote would otherwise break the dump, or run what follows it.
func TestAQuoteInAConnectionValueStaysInsideItsWord(t *testing.T) {
	_, ran := dbLab(t)

	transferDB([]string{"--ssh-user=abc", "--sql-host=db'h", "--sql-user=o'neil",
		"--sql-password=it's; rm -rf ~", "--anonymise=false"}, io.Discard, io.Discard, false)

	want := `-h'db'\''h' -u'o'\''neil' -P 3306 -p'it'\''s; rm -rf ~' main `
	if len(*ran) != 1 || !strings.Contains((*ran)[0].Command(), want) {
		t.Fatalf("ssh ran %v, want it to contain %s", *ran, want)
	}
}
