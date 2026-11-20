package app

import (
	"os"
	"path/filepath"
	"testing"
)

//
// How a domain reaches this machine, as a diagnosis reports it.
//
// The hosts file is a field so that a test can point it at a file of its own: the machine's real
// one is not something a test should read, let alone depend on.
//

// resolverSaying answers ResolvesLocally the same for every name, so the hosts file is the only
// thing that decides between the answers.
type resolverSaying bool

func (r resolverSaying) ResolvesLocally(string) bool { return bool(r) }

func hostsFile(t *testing.T, contents string) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	return file
}

func TestANameInTheHostsFileResolvesThroughIt(t *testing.T) {
	hosts := Hosts{
		File:     hostsFile(t, "127.0.0.1 localhost\n127.0.0.1 tienda.test\n"),
		Resolver: resolverSaying(false),
	}

	if got := hosts.Resolution("tienda.test"); got != "hosts" {
		t.Fatalf("Resolution = %q, want hosts", got)
	}
}

func TestANameThatDoesNotResolveIsReportedAsNone(t *testing.T) {
	hosts := Hosts{
		File:     hostsFile(t, "127.0.0.1 localhost\n"),
		Resolver: resolverSaying(false),
	}

	if got := hosts.Resolution("tienda.test"); got != "none" {
		t.Fatalf("Resolution = %q, want none", got)
	}
}

// TestANameOutsideTheHostsFileThatResolvesLocallyIsDNS: a resolver of its own (dnsmasq, the
// proxy's wildcard) is the second way a name reaches this machine.
func TestANameOutsideTheHostsFileThatResolvesLocallyIsDNS(t *testing.T) {
	hosts := Hosts{
		File:     hostsFile(t, "127.0.0.1 localhost\n"),
		Resolver: resolverSaying(true),
	}

	if got := hosts.Resolution("tienda.test"); got != "dns" {
		t.Fatalf("Resolution = %q, want dns", got)
	}
}

// TestTheHostsFileWinsOverDNS: listed there, the name is reported by the file that answers first.
func TestTheHostsFileWinsOverDNS(t *testing.T) {
	hosts := Hosts{
		File:     hostsFile(t, "127.0.0.1 tienda.test\n"),
		Resolver: resolverSaying(true),
	}

	if got := hosts.Resolution("tienda.test"); got != "hosts" {
		t.Fatalf("Resolution = %q, want hosts", got)
	}
}

// TestAnUnreadableHostsFileFallsBackToDNS: no hosts file is not an error in a diagnosis.
func TestAnUnreadableHostsFileFallsBackToDNS(t *testing.T) {
	hosts := Hosts{
		File:     filepath.Join(t.TempDir(), "missing"),
		Resolver: resolverSaying(true),
	}

	if got := hosts.Resolution("tienda.test"); got != "dns" {
		t.Fatalf("Resolution = %q, want dns", got)
	}
}
