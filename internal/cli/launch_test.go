package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// launch opens one of the project's addresses in the browser: the store by default, or its admin,
// mail catcher, RabbitMQ or search. The address comes from the description; nothing here opens a
// real browser.
//

func launchLab(t *testing.T) (*fakeEngine, *[]string) {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	description := core.Description{}
	description.Project.URLs = core.URLs{
		Base: "https://shop.test/", Admin: "https://shop.test/admin", Mail: "http://127.0.0.1:8025",
		RabbitMQ: "http://127.0.0.1:15672",
	}
	fake.description = description

	opened := &[]string{}
	previousOpen, previousLook := openWith, lookOpener
	t.Cleanup(func() { openWith, lookOpener = previousOpen, previousLook })

	openWith = func(name string, args ...string) error {
		*opened = append(*opened, name+" "+strings.Join(args, " "))

		return nil
	}
	lookOpener = func() (string, bool) { return "open", true }

	return fake, opened
}

func TestLaunchOpensTheAddressAskedFor(t *testing.T) {
	cases := []struct {
		args []string
		url  string
	}{
		{nil, "https://shop.test/"},
		{[]string{"--base"}, "https://shop.test/"},
		{[]string{"--admin"}, "https://shop.test/admin"},
		{[]string{"--mail"}, "http://127.0.0.1:8025"},
		{[]string{"--mailhog"}, "http://127.0.0.1:8025"},
		{[]string{"--mailpit"}, "http://127.0.0.1:8025"},
		{[]string{"--rabbitmq"}, "http://127.0.0.1:15672"},
	}

	for _, tt := range cases {
		_, opened := launchLab(t)

		if code := launch(tt.args, io.Discard, io.Discard, false); code != exitOK {
			t.Fatalf("launch %v = %d, want exitOK", tt.args, code)
		}

		if len(*opened) != 1 || (*opened)[0] != "open "+tt.url {
			t.Fatalf("launch %v opened %v, want [open %s]", tt.args, *opened, tt.url)
		}
	}
}

// TestLaunchUnderJSONSaysWhereAndOpensNothing: a program reading the answer wants the address.
func TestLaunchUnderJSONSaysWhereAndOpensNothing(t *testing.T) {
	_, opened := launchLab(t)

	var stdout bytes.Buffer
	if code := launch([]string{"--admin"}, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("launch --admin --json = %d, want exitOK", code)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}

	want := map[string]any{"target": "admin", "url": "https://shop.test/admin", "opened": false}
	for key, value := range want {
		if envelope.Data[key] != value {
			t.Errorf("data %s = %v, want %v", key, envelope.Data[key], value)
		}
	}

	if len(*opened) != 0 {
		t.Errorf("opened %v under --json", *opened)
	}
}

func TestLaunchRefusesWhatItCannotOpen(t *testing.T) {
	t.Run("two destinations", func(t *testing.T) {
		fake, opened := launchLab(t)

		if code := launch([]string{"--admin", "--mail"}, io.Discard, io.Discard, true); code != exitUsage {
			t.Fatalf("launch --admin --mail = %d, want exitUsage", code)
		}

		if len(fake.calls) != 0 || len(*opened) != 0 {
			t.Errorf("asked %v and opened %v, want nothing", fake.calls, *opened)
		}
	})

	t.Run("an unknown option", func(t *testing.T) {
		_, _ = launchLab(t)

		if code := launch([]string{"--grafana"}, io.Discard, io.Discard, true); code != exitUsage {
			t.Fatalf("launch --grafana = %d, want exitUsage", code)
		}
	})

	t.Run("no domain for the store", func(t *testing.T) {
		fake, opened := launchLab(t)
		fake.description = core.Description{}

		if code := launch(nil, io.Discard, io.Discard, true); code != exitProject {
			t.Fatalf("launch with no domain = %d, want exitProject", code)
		}

		if len(*opened) != 0 {
			t.Errorf("opened %v", *opened)
		}
	})

	t.Run("a service that publishes no port", func(t *testing.T) {
		_, opened := launchLab(t)

		if code := launch([]string{"--search"}, io.Discard, io.Discard, true); code != exitService {
			t.Fatalf("launch --search with no address = %d, want exitService", code)
		}

		if len(*opened) != 0 {
			t.Errorf("opened %v", *opened)
		}
	})
}

// TestWithNoOpenerTheAddressIsPrinted: a machine with no browser still gets the address.
func TestWithNoOpenerTheAddressIsPrinted(t *testing.T) {
	_, opened := launchLab(t)
	lookOpener = func() (string, bool) { return "", false }

	var stdout bytes.Buffer
	if code := launch(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("launch with no opener = %d, want exitOK", code)
	}

	if !strings.Contains(stdout.String(), "https://shop.test/") || len(*opened) != 0 {
		t.Errorf("stdout = %q, opened %v; want the address printed and nothing opened", stdout.String(), *opened)
	}
}

// TestAnOpenerThatFailsIsAFailure: the shell refused with 1 and a hint to ask for the address.
func TestAnOpenerThatFailsIsAFailure(t *testing.T) {
	launchLab(t)
	openWith = func(string, ...string) error { return errors.New("exit status 1") }

	if code := launch(nil, io.Discard, io.Discard, false); code != exitError {
		t.Fatalf("launch with the opener failing = %d, want exitError", code)
	}
}

// TestTheRouterAnswersForLaunch: it reaches its handler, not the shell.
func TestTheRouterAnswersForLaunch(t *testing.T) {
	fake, _ := launchLab(t)

	Run([]string{"launch"}, io.Discard, io.Discard)

	if len(fake.calls) == 0 {
		t.Fatal("launch fell through to the shell implementation")
	}
}
