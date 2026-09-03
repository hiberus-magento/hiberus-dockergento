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
		t.Fatal("la entrada con marca es nuestra")
	}

	if HostsAdded(fichero, "escrita-a-mano.test", "hm") {
		t.Fatal("una escrita a mano no lo es")
	}

	sin := WithoutHost(fichero, "escrita-a-mano.test", "hm")
	if !HostsHas(sin, "escrita-a-mano.test") {
		t.Fatalf("y no se borra:\n%s", sin)
	}

	sin = WithoutHost(fichero, "mia.test", "hm")
	if HostsHas(sin, "mia.test") {
		t.Fatalf("la nuestra sí:\n%s", sin)
	}

	if !HostsHas(sin, "localhost") || !HostsHas(sin, "escrita-a-mano.test") {
		t.Fatalf("y el resto del fichero se queda como estaba:\n%s", sin)
	}
}

// A second line for a name that already resolves is one more line nobody can attribute.
func TestNothingIsAddedTwice(t *testing.T) {
	if WithHost(fichero, "mia.test", "hm") != fichero {
		t.Fatal("no se añade lo que ya está")
	}

	if WithHost(fichero, "escrita-a-mano.test", "hm") != fichero {
		t.Fatal("ni siquiera cuando la escribió otro")
	}

	con := WithHost(fichero, "nueva.test", "hm")
	if !HostsAdded(con, "nueva.test", "hm") {
		t.Fatalf("y la que falta se añade con su marca:\n%s", con)
	}
}

// A name is not an address, and a line may begin with one or with several. Told apart by what the
// field is made of rather than by where it is.
func TestAnAddressIsNotAName(t *testing.T) {
	if HostsHas("127.0.0.1 uno.test", "127.0.0.1") {
		t.Fatal("una dirección no es un nombre")
	}

	if !HostsHas("0.0.0.0 ::1 dos.test", "dos.test") {
		t.Fatal("y el nombre después de dos direcciones sí lo es")
	}
}

// A file with no trailing newline gets one, or the entry would join the last line.
func TestTheEntryIsItsOwnLine(t *testing.T) {
	con := WithHost("127.0.0.1 localhost", "nueva.test", "hm")

	if !HostsHas(con, "localhost") || !HostsHas(con, "nueva.test") {
		t.Fatalf("las dos líneas están:\n%s", con)
	}
}
