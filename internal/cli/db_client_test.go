package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The database launchers: dbeaver, sequelace and tableplus. Ported from console/tasks/db_client.sh,
// which the three shared and which differed between them in one word — the client.
//
// Nothing is ever opened here: openedBy replaces the opener with one that records.
//

// launch is one command the opener was asked to run.
type launch struct {
	Name string
	Args []string
}

// openedBy substitutes the opener for the test and returns what it was asked to run. A non-nil
// err is what every launch answers with.
func openedBy(t *testing.T, err error) *[]launch {
	t.Helper()

	launched := &[]launch{}
	original := openWith
	openWith = func(name string, args ...string) error {
		*launched = append(*launched, launch{Name: name, Args: args})

		return err
	}
	t.Cleanup(func() { openWith = original })

	return launched
}

// dbShop is a running project publishing its database on 13399, with its own credentials.
func dbShop() core.Description {
	description := core.Description{}
	description.Project.Name = "shop"
	description.Services = []core.DescribedService{
		{Name: "db", State: "running", Ports: []string{"13399->3306"}},
		{Name: "phpfpm", State: "running", Ports: []string{}},
	}
	description.Credentials = &core.Credentials{}
	description.Credentials.Database.Name = "tienda_db"
	description.Credentials.Database.User = "tienda"
	description.Credentials.Database.Password = "secreta"

	return description
}

const shopURL = "mysql://tienda:secreta@127.0.0.1:13399/tienda_db"

func answeringFor(t *testing.T, description core.Description) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.description = description
	t.Setenv("NO_COLOR", "1")

	return fake
}

func TestEachLauncherOpensItsClientOnMacOS(t *testing.T) {
	cases := []struct {
		client string
		want   launch
		name   string
	}{
		{"tableplus", launch{"open", []string{"-a", "TablePlus", shopURL}}, "TablePlus"},
		{"sequelace", launch{"open", []string{"-a", "Sequel Ace", shopURL}}, "Sequel Ace"},
		{"dbeaver", launch{"open", []string{"-a", "DBeaver", shopURL}}, "DBeaver"},
	}

	for _, tt := range cases {
		t.Run(tt.client, func(t *testing.T) {
			fake := answeringFor(t, dbShop())
			onPlatform(t, "darwin")
			launched := openedBy(t, nil)
			var stdout bytes.Buffer

			if code := dbClient(tt.client, nil, &stdout, io.Discard, false); code != exitOK {
				t.Fatalf("%s = %d, want exitOK", tt.client, code)
			}

			// The credentials are asked for: without them there is no connection to open
			if diff := cmp.Diff([]call{{Method: "Describe", Dir: here(), WithSecrets: true}}, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff([]launch{tt.want}, *launched); diff != "" {
				t.Fatalf("launched (-want +got):\n%s", diff)
			}

			if got, want := stdout.String(), "Opened "+tt.name+" on shop\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
		})
	}
}

func TestOnLinuxDBeaverIsRunAndTheOthersGoToTheDesktop(t *testing.T) {
	cases := []struct {
		client string
		want   launch
	}{
		{"dbeaver", launch{"dbeaver", []string{shopURL}}},
		{"tableplus", launch{"xdg-open", []string{shopURL}}},
		{"sequelace", launch{"xdg-open", []string{shopURL}}},
	}

	for _, tt := range cases {
		t.Run(tt.client, func(t *testing.T) {
			answeringFor(t, dbShop())
			onPlatform(t, "linux")
			launched := openedBy(t, nil)

			if code := dbClient(tt.client, nil, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("%s = %d, want exitOK", tt.client, code)
			}

			if diff := cmp.Diff([]launch{tt.want}, *launched); diff != "" {
				t.Fatalf("launched (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAPortGivenOverridesThePublishedOne(t *testing.T) {
	for _, args := range [][]string{{"--port=40000", "--print"}, {"--port", "40000", "--print"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			answeringFor(t, dbShop())
			launched := openedBy(t, nil)
			var stdout bytes.Buffer

			if code := dbClient("tableplus", args, &stdout, io.Discard, false); code != exitOK {
				t.Fatalf("tableplus %v = %d, want exitOK", args, code)
			}

			if got, want := stdout.String(), "mysql://tienda:secreta@127.0.0.1:40000/tienda_db\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}

			if len(*launched) != 0 {
				t.Fatalf("opened %v, want nothing", *launched)
			}
		})
	}
}

func TestCredentialsNobodySetAreMagentos(t *testing.T) {
	description := dbShop()
	description.Credentials = &core.Credentials{}
	answeringFor(t, description)
	openedBy(t, nil)
	var stdout bytes.Buffer

	if code := dbClient("dbeaver", []string{"--print"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("dbeaver --print = %d, want exitOK", code)
	}

	if got, want := stdout.String(), "mysql://magento:magento@127.0.0.1:13399/magento\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintPrintsTheConnectionAndOpensNothing(t *testing.T) {
	answeringFor(t, dbShop())
	launched := openedBy(t, nil)
	var stdout bytes.Buffer

	if code := dbClient("sequelace", []string{"--print"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("sequelace --print = %d, want exitOK", code)
	}

	if got := stdout.String(); got != shopURL+"\n" {
		t.Fatalf("stdout = %q, want exactly the URL and a newline", got)
	}

	if len(*launched) != 0 {
		t.Fatalf("opened %v, want nothing", *launched)
	}
}

func TestAsDataTheConnectionIsFieldsAndNothingIsOpened(t *testing.T) {
	answeringFor(t, dbShop())
	launched := openedBy(t, nil)
	var stdout bytes.Buffer

	if code := dbClient("tableplus", nil, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("tableplus under --json = %d, want exitOK", code)
	}

	var got struct {
		Command string            `json:"command"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("not a document: %v\n%s", err, stdout.String())
	}

	want := map[string]string{
		"url": shopURL, "host": "127.0.0.1", "port": "13399", "user": "tienda", "database": "tienda_db",
	}
	if diff := cmp.Diff(want, got.Data); diff != "" {
		t.Fatalf("data (-want +got):\n%s", diff)
	}

	if got.Command != "tableplus" {
		t.Fatalf("command = %q, want tableplus", got.Command)
	}

	// The port is a string, as `jq --arg` made it
	if !strings.Contains(stdout.String(), `"port": "13399"`) {
		t.Fatalf("port is not the string the shell produced:\n%s", stdout.String())
	}

	if len(*launched) != 0 {
		t.Fatalf("opened %v, want nothing", *launched)
	}
}

func TestADatabaseNothingCanReachIsRefused(t *testing.T) {
	t.Setenv("COMMAND_BIN_NAME", "hm")
	description := dbShop()
	description.Services[0].Ports = []string{}
	answeringFor(t, description)
	launched := openedBy(t, nil)
	var stderr bytes.Buffer

	if code := dbClient("tableplus", nil, io.Discard, &stderr, true); code != exitBlocked {
		t.Fatalf("tableplus with no published port = %d, want %d", code, exitBlocked)
	}

	if !strings.Contains(stderr.String(), `"no_published_port"`) {
		t.Fatalf("stderr lacks no_published_port:\n%s", stderr.String())
	}

	// As text, where the hint is not JSON-escaped
	stderr.Reset()
	dbClient("tableplus", nil, io.Discard, &stderr, false)

	for _, want := range []string{
		"This project publishes no database port, so nothing on this machine can reach it",
		"hm tunnel db, and then hm tableplus --port=<the port it prints>",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}

	if len(*launched) != 0 {
		t.Fatalf("opened %v, want nothing", *launched)
	}
}

// TestAPortPublishedForAnotherTargetIsNotTheDatabases: only the container's 3306 counts.
func TestAPortPublishedForAnotherTargetIsNotTheDatabases(t *testing.T) {
	description := dbShop()
	description.Services[0].Ports = []string{"13399->3307"}
	answeringFor(t, description)
	openedBy(t, nil)

	if code := dbClient("tableplus", []string{"--print"}, io.Discard, io.Discard, false); code != exitBlocked {
		t.Fatalf("tableplus = %d, want %d", code, exitBlocked)
	}
}

func TestADatabaseThatIsNotRunningIsSaidSo(t *testing.T) {
	description := dbShop()
	description.Services[0].State = "exited"
	answeringFor(t, description)
	launched := openedBy(t, nil)
	var stderr bytes.Buffer

	if code := dbClient("dbeaver", []string{"--port=1"}, io.Discard, &stderr, true); code != exitService {
		t.Fatalf("dbeaver with db stopped = %d, want %d", code, exitService)
	}

	if !strings.Contains(stderr.String(), `"service_not_running"`) {
		t.Fatalf("stderr lacks service_not_running:\n%s", stderr.String())
	}

	if len(*launched) != 0 {
		t.Fatalf("opened %v, want nothing", *launched)
	}
}

func TestAnOptionNobodyDeclaredIsAUsageErrorAskingNothing(t *testing.T) {
	t.Setenv("COMMAND_BIN_NAME", "hm")
	fake := answeringFor(t, dbShop())
	launched := openedBy(t, nil)
	var stderr bytes.Buffer

	if code := dbClient("tableplus", []string{"--nonsense"}, io.Discard, &stderr, true); code != exitUsage {
		t.Fatalf("tableplus --nonsense = %d, want %d", code, exitUsage)
	}

	if !strings.Contains(stderr.String(), `"invalid_argument"`) {
		t.Fatalf("stderr lacks invalid_argument:\n%s", stderr.String())
	}

	// As text, where the hint is not JSON-escaped
	stderr.Reset()
	dbClient("tableplus", []string{"--nonsense"}, io.Discard, &stderr, false)

	for _, want := range []string{"Unknown option: --nonsense", "hm tableplus [--print] [--port=<port>]"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}

	if len(fake.calls) != 0 || len(*launched) != 0 {
		t.Fatalf("asked %v and opened %v, want nothing", fake.calls, *launched)
	}
}

func TestAClientThatCannotBeOpenedStillGivesTheConnection(t *testing.T) {
	answeringFor(t, dbShop())
	onPlatform(t, "darwin")
	openedBy(t, errors.New("Unable to find application named 'TablePlus'"))
	var stdout, stderr bytes.Buffer

	if code := dbClient("tableplus", nil, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("tableplus not installed = %d, want exitOK", code)
	}

	if got, want := stderr.String(), "TablePlus could not be opened; it may not be installed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}

	if got, want := stdout.String(), "  The connection:\n\n    "+shopURL+"\n\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestTheRouterAnswersForTheDatabaseLaunchers(t *testing.T) {
	for _, client := range []string{"dbeaver", "sequelace", "tableplus"} {
		t.Run(client, func(t *testing.T) {
			fake := answeringFor(t, dbShop())
			openedBy(t, nil)

			Run([]string{client, "--print"}, io.Discard, io.Discard)

			// An unrouted command falls through to the legacy runner, which never reaches the fake
			if len(fake.calls) == 0 {
				t.Fatalf("%s asked the fake engine nothing, want the router to reach the handler", client)
			}
		})
	}
}
