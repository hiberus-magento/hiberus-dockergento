package machine

import "testing"

//
// Whether a name reaches this machine.
//
// Only answers that cannot leave the machine are asked for here: localhost is in every hosts file,
// and a name under .invalid is reserved never to resolve (RFC 6761).
//

func TestLocalhostResolvesLocally(t *testing.T) {
	if !(Resolver{}).ResolvesLocally("localhost") {
		t.Fatal("ResolvesLocally(localhost) = false, want true")
	}
}

func TestAnEmptyNameDoesNotResolve(t *testing.T) {
	// A project with no domain must never look reachable, whatever the system resolver would make
	// of an empty name
	if (Resolver{}).ResolvesLocally("") {
		t.Fatal(`ResolvesLocally("") = true, want false`)
	}
}

func TestANameThatDoesNotResolveIsNotLocal(t *testing.T) {
	if (Resolver{}).ResolvesLocally("tienda.hm-test.invalid") {
		t.Fatal("ResolvesLocally(a name under .invalid) = true, want false")
	}
}

// An internet address is somebody else's machine. Checked on the addresses themselves, so no name
// has to be looked up to prove it.
func TestOnlyLoopbackAddressesCountAsThisMachine(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":     true,
		"127.0.1.1":     true,
		"::1":           true,
		"93.184.216.34": false,
		"192.168.1.10":  false,
		"2001:db8::1":   false,
		"":              false,
	}

	for address, want := range cases {
		if got := loopback(address); got != want {
			t.Errorf("loopback(%q) = %v, want %v", address, got, want)
		}
	}
}
