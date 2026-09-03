package e2e_test

import (
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/test/e2e"
)

//
// Pointing a domain at this machine.
//
// Every entry in the hosts file costs a password prompt, and they were never removed — twenty-three
// of them on the machine this was written on, several from projects that no longer exist. So two
// things have to be true: nothing is added when the name already resolves here, and what is added
// carries a marker so the tool can find its own and leave alone what a person wrote.
//
// The domains here end in `.invalid`, which is reserved never to resolve. A `.test` would have done
// on most machines and not on one with a wildcard resolver for it — which is exactly the case the
// command exists to notice, and it would have looked like the command doing nothing.
//

// hosts is a session whose machine already has a hosts file, and a project to run in.
func hosts(t *testing.T, contents string) (*e2e.Session, e2e.Project) {
	t.Helper()

	session := e2e.New(t)
	session.Writes(t, session.HostsFile, contents)

	return session, e2e.NewProject(t, "hm-e2e-hosts", e2e.Definition{})
}

func TestSetHostAddsAnEntry(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "127.0.0.1 localhost\n255.255.255.255 broadcasthost\n")

	got := session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	if got.Code != 0 {
		t.Fatalf("set-host tienda.invalid = %d, want 0\n%s", got.Code, got.Output())
	}

	written := session.Reads(session.HostsFile)

	for _, want := range []string{
		"tienda.invalid",

		// The marker is the whole point of it: an entry with nothing to say where it came from
		// accumulates for as long as the machine lives and nobody dares delete one
		"# added by",

		// And what was already in the file is somebody else's
		"127.0.0.1 localhost",
		"broadcasthost",
	} {
		if !strings.Contains(written, want) {
			t.Errorf("hosts file = %q, want it to contain %q", written, want)
		}
	}
}

// A second line for a name that already resolves is one more line nobody can attribute.
func TestSetHostAddsNothingTwice(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	for range 2 {
		session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")
	}

	written := session.Reads(session.HostsFile)

	if got := strings.Count(written, "tienda.invalid"); got != 1 {
		t.Errorf("entries for tienda.invalid = %d, want 1\n%s", got, written)
	}
}

// `https://shop.invalid/` is a name with punctuation around it, and what goes in a hosts file is
// the name.
func TestSetHostWritesANameAndNotAURL(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	session.Run(t, project.Root, "--no-json", "set-host", "https://otra.invalid/", "--no-database")

	written := session.Reads(session.HostsFile)

	if !strings.Contains(written, " otra.invalid ") {
		t.Errorf("hosts file = %q, want it to contain the name otra.invalid", written)
	}

	if strings.Contains(written, "https://") {
		t.Errorf("hosts file = %q, want no scheme in it", written)
	}
}

// A line somebody wrote by hand has no marker, so it is not this tool's to delete. That is the
// whole reason the marker exists.
func TestSetHostRemovesOnlyWhatItAdded(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "127.0.0.1 localhost\n127.0.0.1 escrita-a-mano.invalid\n")
	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	t.Run("one it did not add", func(t *testing.T) {
		got := session.Run(t, project.Root, "--no-json", "set-host", "--remove",
			"escrita-a-mano.invalid")

		if written := session.Reads(session.HostsFile); !strings.Contains(written, "escrita-a-mano.invalid") {
			t.Errorf("hosts file = %q, want the hand-written entry left in it", written)
		}

		if !strings.Contains(got.Output(), "no entry") {
			t.Errorf("set-host --remove escrita-a-mano.invalid said %q, want it to say there is no entry",
				got.Output())
		}
	})

	t.Run("one it did add", func(t *testing.T) {
		session.Run(t, project.Root, "--no-json", "set-host", "--remove", "tienda.invalid")

		written := session.Reads(session.HostsFile)

		if strings.Contains(written, "tienda.invalid") {
			t.Errorf("hosts file = %q, want its own entry gone", written)
		}

		for _, want := range []string{"127.0.0.1 localhost", "escrita-a-mano.invalid"} {
			if !strings.Contains(written, want) {
				t.Errorf("hosts file = %q, want %q still in it", written, want)
			}
		}
	})
}

func TestSetHostRefusals(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	cases := map[string]struct {
		args []string
		want int
	}{
		"removing nothing in particular": {args: []string{"set-host", "--remove"}, want: 2},
		"an option nobody declared":      {args: []string{"set-host", "--tonteria"}, want: 2},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := session.Run(t, project.Root, one.args...); got.Code != one.want {
				t.Errorf("%v = %d, want %d\n%s", one.args, got.Code, one.want, got.Output())
			}
		})
	}
}

// A wildcard resolver for the TLD — ours, or one the machine already had — makes the entry
// pointless, and the entry is what costs a password prompt and stays for ever. `localhost` resolves
// on every machine there is, which is what makes it the case to check.
func TestSetHostLeavesAloneWhatAlreadyResolves(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	got := session.Run(t, project.Root, "--no-json", "set-host", "localhost", "--no-database")

	if written := session.Reads(session.HostsFile); written != "" {
		t.Errorf("hosts file = %q, want it untouched", written)
	}

	if !strings.Contains(got.Output(), "already resolves") {
		t.Errorf("set-host localhost said %q, want it to say the name already resolves", got.Output())
	}
}

// The base URLs are a different thing that fails differently: one needs the system password and
// touches a file every program reads, and the other is a row in the project's database.
func TestSetHostWritesTheBaseURLs(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	// A shell tree that installs nothing and writes down what it was asked to do: what has to be
	// right here is that Magento is asked for the right thing, not that Magento answers
	asked := e2e.Recorder(t, session)

	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid")

	for _, want := range []string{
		"magento config:set web/secure/base_url https://tienda.invalid/",
		"magento config:set web/unsecure/base_url https://tienda.invalid/",
	} {
		if got := asked(); !strings.Contains(got, want) {
			t.Errorf("asked of the shell half = %q, want it to contain %q", got, want)
		}
	}
}

func TestSetHostAsksNothingOfMagentoWhenToldNotTo(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")
	asked := e2e.Recorder(t, session)

	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	if got := asked(); got != "" {
		t.Errorf("asked of the shell half = %q, want nothing", got)
	}
}
