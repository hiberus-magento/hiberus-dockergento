package core

import (
	"strings"
	"testing"
)

const fichero = `127.0.0.1 localhost
0.0.0.0 ::1 mia.test # added by hm
127.0.0.1 escrita-a-mano.test
::1 otra.test # added by hm
`

// What this tool added is its to remove. What somebody wrote by hand is not, however much it looks
// like ours.
func TestOnlyWhatThisToolAddedIsRemoved(t *testing.T) {
	if !HostsAdded(fichero, "mia.test", "hm") {
		t.Fatal("HostsAdded(marked entry) = false, want true")
	}

	if HostsAdded(fichero, "escrita-a-mano.test", "hm") {
		t.Fatal("HostsAdded(hand-written entry) = true, want false")
	}

	sin := WithoutHost(fichero, "escrita-a-mano.test", "hm")
	if !HostsHas(sin, "escrita-a-mano.test") {
		t.Fatalf("file without the hand-written entry = %q, want it kept", sin)
	}

	sin = WithoutHost(fichero, "mia.test", "hm")
	if HostsHas(sin, "mia.test") {
		t.Fatalf("file with our entry removed = %q, want it gone", sin)
	}

	if !HostsHas(sin, "localhost") || !HostsHas(sin, "escrita-a-mano.test") {
		t.Fatalf("file after removing ours = %q, want the rest of it untouched", sin)
	}
}

// A second line for a name that already resolves is one more line nobody can attribute.
func TestNothingIsAddedTwice(t *testing.T) {
	if WithHost(fichero, "otra.test", "hm") != fichero {
		t.Fatal("WithHost(name already there) changed the file, want it left alone")
	}

	if WithHost(fichero, "escrita-a-mano.test", "hm") != fichero {
		t.Fatal("WithHost(name somebody else wrote) changed the file, want it left alone")
	}

	con := WithHost(fichero, "nueva.test", "hm")
	if !HostsAdded(con, "nueva.test", "hm") {
		t.Fatalf("file with a new entry = %q, want it added with the marker", con)
	}
}

// A name is not an address, and a line may begin with one or with several. Told apart by what the
// field is made of rather than by where it is.
func TestAnAddressIsNotAName(t *testing.T) {
	if HostsHas("127.0.0.1 uno.test", "127.0.0.1") {
		t.Fatal("HostsHas(file, an address) = true, want false")
	}

	if !HostsHas("0.0.0.0 ::1 dos.test", "dos.test") {
		t.Fatal("HostsHas(file, a name after two addresses) = false, want true")
	}
}

// A file with no trailing newline gets one, or the entry would join the last line.
func TestTheEntryIsItsOwnLine(t *testing.T) {
	con := WithHost("127.0.0.1 localhost", "nueva.test", "hm")

	if !HostsHas(con, "localhost") || !HostsHas(con, "nueva.test") {
		t.Fatalf("file = %q, want both lines in it", con)
	}
}

// The entry this tool wrote until 1.5.0 was a single `0.0.0.0 ::1 <domain>` line, and a hosts line
// carries one address: `::1` was read as an alias, so the domain resolved to 0.0.0.0 only — an
// address Firefox refuses to connect to, while Chrome and curl reach it and hide the whole thing.
//
// Asking whether the domain resolves would leave that line exactly where it is, because it does
// resolve: to 0.0.0.0. So the malformed entry is a question of its own, and the answer heals the
// file the next time the tool is asked for the domain.
func TestTheMalformedEntryIsRepaired(t *testing.T) {
	if !HostsHasLegacy(fichero, "mia.test") {
		t.Fatal("HostsHasLegacy(the 0.0.0.0 entry) = false, want true")
	}

	repaired := WithHost(fichero, "mia.test", "hm")

	if strings.Contains(repaired, "0.0.0.0") {
		t.Fatalf("repaired file = %q, want the 0.0.0.0 entry gone", repaired)
	}

	for _, want := range []string{"127.0.0.1 mia.test # added by hm", "::1 mia.test # added by hm"} {
		if !strings.Contains(repaired, want) {
			t.Fatalf("repaired file = %q, want %q in it", repaired, want)
		}
	}

	// And the rest of the file was never ours
	for _, want := range []string{"127.0.0.1 localhost", "127.0.0.1 escrita-a-mano.test", "::1 otra.test"} {
		if !strings.Contains(repaired, want) {
			t.Fatalf("repaired file = %q, want %q untouched", repaired, want)
		}
	}
}

// One line per address family, both marked, because the removal path finds its own by the marker.
func TestTheEntriesAreOnePerAddressFamily(t *testing.T) {
	entries := HostsEntries("nueva.test", "hm")

	if got := strings.Count(strings.TrimSuffix(entries, "\n"), "\n"); got != 1 {
		t.Fatalf("HostsEntries = %q, want two lines", entries)
	}

	for _, want := range []string{"127.0.0.1 nueva.test # added by hm", "::1 nueva.test # added by hm"} {
		if !strings.Contains(entries, want) {
			t.Fatalf("HostsEntries = %q, want %q in it", entries, want)
		}
	}
}

// A malformed entry belongs to whoever's domain it names, and repairing one is not licence to
// touch another.
func TestAnotherDomainsMalformedEntryIsLeftAlone(t *testing.T) {
	if HostsHasLegacy(fichero, "nueva.test") {
		t.Fatal("HostsHasLegacy(a domain with no entry) = true, want false")
	}

	added := WithHost(fichero, "nueva.test", "hm")

	if !strings.Contains(added, "0.0.0.0 ::1 mia.test") {
		t.Fatalf("file = %q, want another domain's entry untouched", added)
	}
}

// What a caller asks about is the domain, and there are two lines per domain now — so a domain is
// named once, however many lines carry it. Otherwise the offer to clean one up is made twice.
func TestOursAreListedOncePerDomain(t *testing.T) {
	file := WithHost(WithHost("127.0.0.1 localhost\n", "una.test", "hm"), "otra.test", "hm")

	marked := HostsMarked(file, HostsMarker("hm"))

	if len(marked) != 2 {
		t.Fatalf("HostsMarked = %v, want the two domains once each", marked)
	}

	if marked[0] != "una.test" || marked[1] != "otra.test" {
		t.Fatalf("HostsMarked = %v, want them in the order the file has them", marked)
	}

	// A line somebody wrote by hand has no marker, so it is not ours to offer
	if marked := HostsMarked("127.0.0.1 escrita-a-mano.test\n", HostsMarker("hm")); len(marked) != 0 {
		t.Fatalf("HostsMarked(file with no entry of ours) = %v, want none", marked)
	}
}
