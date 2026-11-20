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

//
// The overlay that hands a project to the proxy. What it writes is decided here; whether Compose
// and Traefik do what it says is the live check's business.
//

// overlayServices is every service block under `services:` in an overlay, by name, and the names
// in the order they are written. The overlay is flat YAML of our own making, so indentation is
// enough to read it; the networks after the services are not services.
func overlayServices(overlay string) ([]string, map[string]string) {
	var order []string

	blocks := map[string]string{}
	inServices := false
	current := ""

	for _, line := range strings.Split(overlay, "\n") {
		switch {
		case !strings.HasPrefix(line, " "):
			inServices = line == "services:"
		case !inServices:
		case !strings.HasPrefix(line, "   "):
			current = strings.TrimSuffix(strings.Fields(line)[0], ":")
			order = append(order, current)
			blocks[current] += line + "\n"
		default:
			blocks[current] += line + "\n"
		}
	}

	return order, blocks
}

// A project behind the proxy publishes nothing: the ports are the proxy's, and a second project
// holding them is the clash the proxy exists to end.
func TestEveryServiceInTheOverlayStopsPublishingPorts(t *testing.T) {
	_, blocks := overlayServices(ProxyOverlay("hm", "shop", "shop.local", ""))

	for _, service := range []string{"phpfpm", "nginx", "db", "redis", "varnish", "rabbitmq", "mailhog", "search"} {
		if !strings.Contains(blocks[service], "ports: !reset []") {
			t.Errorf("%s in the overlay = %q, want its ports reset", service, blocks[service])
		}
	}
}

// A repeated key in YAML is not a merge: the last block wins and the earlier one disappears
// without a word, which is how services quietly kept their ports the first time.
func TestNoServiceIsDeclaredTwiceInTheOverlay(t *testing.T) {
	order, _ := overlayServices(ProxyOverlay("hm", "shop", "shop.local", ""))

	seen := map[string]bool{}

	for _, service := range order {
		if seen[service] {
			t.Fatalf("service %q is declared twice in the overlay, want once", service)
		}

		seen[service] = true
	}
}

// Hitch only gives Varnish the HTTPS the proxy now terminates, so it is removed outright.
func TestTheOverlayRemovesHitch(t *testing.T) {
	if overlay := ProxyOverlay("hm", "shop", "shop.local", ""); !strings.Contains(overlay, "  hitch: !reset null\n") {
		t.Fatalf("overlay = %q, want hitch reset to null", overlay)
	}
}

// The auxiliary interfaces live on subdomains: served under a path prefix most of them generate
// absolute links and break.
func TestTheOverlayRoutesTheShopAndItsInterfacesByHost(t *testing.T) {
	overlay := ProxyOverlay("hm", "shop", "shop.local", "")

	for _, host := range []string{"shop.local", "mail.shop.local", "queue.shop.local", "search.shop.local"} {
		if !strings.Contains(overlay, "Host(`"+host+"`)") {
			t.Errorf("overlay has no router for %s, want one in:\n%s", host, overlay)
		}
	}
}

func TestTheChosenMailCatcherIsTheOneRouted(t *testing.T) {
	order, blocks := overlayServices(ProxyOverlay("hm", "shop", "shop.local", "mailpit"))

	if !strings.Contains(blocks["mailpit"], "Host(`mail.shop.local`)") {
		t.Fatalf("mailpit in the overlay = %q, want it routed at mail.shop.local", blocks["mailpit"])
	}

	for _, service := range order {
		if service == "mailhog" {
			t.Fatal("the overlay for mailpit declares mailhog, want only the chosen catcher")
		}
	}
}

// The proxy reaches a container only through the shared network, so a routed service off it is a
// router pointing at nothing.
func TestRoutedServicesJoinTheGatewayNetwork(t *testing.T) {
	_, blocks := overlayServices(ProxyOverlay("hm", "shop", "shop.local", ""))

	for _, service := range []string{"varnish", "mailhog", "rabbitmq", "search"} {
		if !strings.Contains(blocks[service], "- "+ProxyNetwork+"\n") {
			t.Errorf("%s in the overlay = %q, want it on %s", service, blocks[service], ProxyNetwork)
		}
	}
}
