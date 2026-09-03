package core

import (
	"strings"
	"testing"
)

//
// The proxy's compose file is generated rather than shipped, so what it says is decided here.
//

func TestTheProxyFileSaysTheThingsItHasTo(t *testing.T) {
	written := ProxyCompose("hm")

	for _, wanted := range []string{
		// Pinned: a proxy that upgrades itself underneath a machine full of running projects is
		// a morning spent finding out why
		"image: " + ProxyImage,
		"container_name: " + ProxyContainer,
		`- "80:80"`,
		`- "443:443"`,
		// The dashboard exposes what is routed where, which is nobody else's business on a
		// shared network
		`- "127.0.0.1:8080:8080"`,
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.web.http.redirections.entrypoint.to=websecure",
		"name: " + ProxyNetwork,
		"external: true",
	} {
		if !strings.Contains(written, wanted) {
			t.Fatalf("falta %q en:\n%s", wanted, written)
		}
	}
}

// A repeated key in YAML is not a merge: the last one wins and the earlier block disappears
// without a word.
func TestNoKeyIsWrittenTwice(t *testing.T) {
	seen := map[string]bool{}

	for _, line := range strings.Split(ProxyCompose("hm"), "\n") {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") ||
			!strings.HasSuffix(strings.TrimSpace(line), ":") {
			continue
		}

		key := strings.TrimSpace(line)
		if seen[key] {
			t.Fatalf("key %q is written twice, want once", key)
		}

		seen[key] = true
	}
}

// The version this refuses is the one where `!reset` does not exist, and `!reset` is what lets a
// project drop its published ports without a second template to maintain.
func TestWhichComposeIsNewEnough(t *testing.T) {
	for version, wanted := range map[string]bool{
		"2.24.0":   true,
		"v2.40.3":  true,
		"2.23.9":   false,
		"3.0.0":    true,
		"1.29.2":   false,
		"":         false,
		"nonsense": false,
	} {
		if got := ComposeAtLeast(version, ProxyMinCompose); got != wanted {
			t.Fatalf("%q: se acepta %v y debería ser %v", version, got, wanted)
		}
	}
}
