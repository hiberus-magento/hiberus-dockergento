package core

import "testing"

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
	if WithHost(fichero, "mia.test", "hm") != fichero {
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
