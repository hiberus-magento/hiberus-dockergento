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

	return session, e2e.NewProject(t, "hm-e2e-hosts", e2e.Compose{})
}

func TestAnEntryIsAddedWithAMarkerSayingWhoAddedIt(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "127.0.0.1 localhost\n255.255.255.255 broadcasthost\n")

	result := session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	if result.Code != 0 {
		t.Fatalf("salió con %d: %s", result.Code, result.Output())
	}

	written := session.Reads(session.HostsFile)

	if !strings.Contains(written, "tienda.invalid") || !strings.Contains(written, "# added by") {
		t.Fatalf("la entrada no está, o no dice quién la puso:\n%s", written)
	}

	// What was already in the file is somebody else's, including the lines every machine has
	if !strings.Contains(written, "127.0.0.1 localhost") ||
		!strings.Contains(written, "broadcasthost") {
		t.Fatalf("y lo que ya estaba se queda:\n%s", written)
	}
}

// A second line for a name that already resolves is one more line nobody can attribute.
func TestNothingIsAddedTwice(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	for i := 0; i < 2; i++ {
		session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")
	}

	written := session.Reads(session.HostsFile)

	if count := strings.Count(written, "tienda.invalid"); count != 1 {
		t.Fatalf("se escribió %d veces:\n%s", count, written)
	}
}

// `https://shop.invalid/` is a name with punctuation around it, and what goes in a hosts file is
// the name.
func TestADomainTypedAsAURLIsWrittenAsAName(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	session.Run(t, project.Root, "--no-json", "set-host", "https://otra.invalid/", "--no-database")

	written := session.Reads(session.HostsFile)

	if !strings.Contains(written, " otra.invalid ") || strings.Contains(written, "https://") {
		t.Fatalf("se escribió la URL en vez del nombre:\n%s", written)
	}
}

//
// A line somebody wrote by hand has no marker, so it is not this tool's to delete. That is the
// whole reason the marker exists.
//
func TestOnlyWhatThisToolAddedIsRemoved(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "127.0.0.1 localhost\n127.0.0.1 escrita-a-mano.invalid\n")

	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	result := session.Run(t, project.Root, "--no-json", "set-host", "--remove",
		"escrita-a-mano.invalid")

	if !strings.Contains(session.Reads(session.HostsFile), "escrita-a-mano.invalid") {
		t.Fatalf("una entrada ajena no se borra:\n%s", session.Reads(session.HostsFile))
	}

	if !strings.Contains(result.Output(), "no entry") {
		t.Fatalf("y se dice por qué: %s", result.Output())
	}

	session.Run(t, project.Root, "--no-json", "set-host", "--remove", "tienda.invalid")

	written := session.Reads(session.HostsFile)

	if strings.Contains(written, "tienda.invalid") {
		t.Fatalf("la nuestra sí se borra:\n%s", written)
	}

	if !strings.Contains(written, "127.0.0.1 localhost") ||
		!strings.Contains(written, "escrita-a-mano.invalid") {
		t.Fatalf("y el resto del fichero queda exactamente como estaba:\n%s", written)
	}
}

func TestRemovingNothingInParticularIsARefusal(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	if result := session.Run(t, project.Root, "set-host", "--remove"); result.Code != 2 {
		t.Fatalf("salió con %d: %s", result.Code, result.Output())
	}
}

//
// A wildcard resolver for the TLD — ours, or one the machine already had — makes the entry
// pointless, and the entry is what costs a password prompt and stays for ever. `localhost` resolves
// on every machine there is, which is what makes it the case to check.
//
func TestANameThatAlreadyResolvesHereIsLeftAlone(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	result := session.Run(t, project.Root, "--no-json", "set-host", "localhost", "--no-database")

	if written := session.Reads(session.HostsFile); written != "" {
		t.Fatalf("no se escribe nada:\n%s", written)
	}

	if !strings.Contains(result.Output(), "already resolves") {
		t.Fatalf("y se dice: %s", result.Output())
	}
}

//
// The base URLs are a different thing that fails differently: one needs the system password and
// touches a file every program reads, and the other is a row in the project's database.
//
func TestTheBaseURLsAreWrittenToTheDatabase(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")

	// A shell tree that installs nothing and writes down what it was asked to do: what has to be
	// right here is that Magento is asked for the right thing, not that Magento answers
	recorder := e2e.Recorder(t, session)

	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid")

	asked := recorder()

	for _, wanted := range []string{
		"magento config:set web/secure/base_url https://tienda.invalid/",
		"magento config:set web/unsecure/base_url https://tienda.invalid/",
	} {
		if !strings.Contains(asked, wanted) {
			t.Fatalf("falta %q en:\n%s", wanted, asked)
		}
	}
}

func TestWithNoDatabaseNothingIsAskedOfMagento(t *testing.T) {
	t.Parallel()

	session, project := hosts(t, "")
	recorder := e2e.Recorder(t, session)

	session.Run(t, project.Root, "--no-json", "set-host", "tienda.invalid", "--no-database")

	if asked := recorder(); asked != "" {
		t.Fatalf("no se pide nada:\n%s", asked)
	}
}
