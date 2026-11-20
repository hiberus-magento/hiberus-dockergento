package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// tunnel's command line: its arguments, what it prints, and that the relay is held and then
// closed. What it asks of Docker is the use case's, held in dockergento/app/tunnel_test.go.
//

func tunnelingShop(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop"}
	fake.tunnel = core.Tunnel{Service: "db", Host: "127.0.0.1", Port: 54321, ContainerPort: 3306,
		Relay: "hm-tunnel-shop-db"}
	t.Setenv("NO_COLOR", "1")

	return fake
}

func TestTunnelOpensHoldsAndClosesTheServicesRelay(t *testing.T) {
	fake := tunnelingShop(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "tunnel", "db"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("tunnel db = %d\nstderr: %s", code, stderr.String())
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}

	want := map[string]any{"service": "db", "host": "127.0.0.1", "port": 54321.0, "container_port": 3306.0}
	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Errorf("data (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"Resolve", "Tunnel db 0", "HoldTunnel hm-tunnel-shop-db", "CloseTunnels db"},
		tunnelCalls(fake)); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}

	if !strings.Contains(stderr.String(), "Tunnel closed.") {
		t.Errorf("stderr = %q, want it to say the tunnel closed", stderr.String())
	}
}

func TestTunnelTakesThePortToForward(t *testing.T) {
	fake := tunnelingShop(t)

	Run([]string{"--json", "tunnel", "db", "33060"}, &bytes.Buffer{}, &bytes.Buffer{})

	if got := tunnelCalls(fake); len(got) < 2 || got[1] != "Tunnel db 33060" {
		t.Errorf("calls = %v, want port 33060", got)
	}
}

func TestTunnelSaysWhereInText(t *testing.T) {
	tunnelingShop(t)

	var stdout bytes.Buffer
	Run([]string{"--no-json", "tunnel", "db"}, &stdout, &bytes.Buffer{})

	for _, line := range []string{
		"db is reachable at 127.0.0.1:54321\n\n",
		"  Leave this running while you use it. Ctrl-C closes it.\n",
		"Tunnel closed.\n",
	} {
		if !strings.Contains(stdout.String(), line) {
			t.Errorf("stdout lacks %q:\n%s", line, stdout.String())
		}
	}
}

func TestTunnelCloseClosesTheServicesOrEveryOne(t *testing.T) {
	for _, tt := range []struct {
		args    []string
		service string
		open    bool
		want    string
	}{
		{[]string{"tunnel", "--close"}, "", true, "Tunnels closed.\n"},
		{[]string{"tunnel", "db", "--close"}, "db", false, "No tunnels were open.\n"},
	} {
		fake := tunnelingShop(t)
		fake.tunnelsClosed = tt.open

		var stdout bytes.Buffer
		if code := Run(append([]string{"--no-json"}, tt.args...), &stdout, &bytes.Buffer{}); code != exitOK {
			t.Fatalf("%v = %d", tt.args, code)
		}

		if stdout.String() != tt.want {
			t.Errorf("%v said %q, want %q", tt.args, stdout.String(), tt.want)
		}

		if diff := cmp.Diff([]string{"Resolve", "CloseTunnels " + tt.service}, tunnelCalls(fake)); diff != "" {
			t.Errorf("%v calls (-want +got):\n%s", tt.args, diff)
		}
	}
}

func TestTunnelRefusesAPortThatIsNotANumber(t *testing.T) {
	fake := tunnelingShop(t)

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "tunnel", "db", "mysql"}, &bytes.Buffer{}, &stderr); code != exitUsage {
		t.Fatalf("tunnel db mysql = %d, want exitUsage", code)
	}

	if !strings.Contains(stderr.String(), `"invalid_argument"`) {
		t.Errorf("stderr = %s", stderr.String())
	}

	if diff := cmp.Diff([]string{"Resolve"}, tunnelCalls(fake)); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

func tunnelCalls(fake *fakeEngine) []string {
	described := make([]string, 0, len(fake.calls))

	for _, one := range fake.calls {
		switch one.Method {
		case "Tunnel":
			described = append(described, "Tunnel "+one.Service+" "+strconv.Itoa(one.Port))
		case "CloseTunnels":
			described = append(described, "CloseTunnels "+one.Service)
		case "HoldTunnel":
			described = append(described, "HoldTunnel "+one.Key)
		default:
			described = append(described, one.Method)
		}
	}

	return described
}

// TestTunnelWithoutAServiceIsNotTakenDownByItsOwnInterruptionHandling: without a service no
// interruption is taken (closing "every tunnel" would be wrong), and an engine that opens anyway
// must not find a missing handler.
func TestTunnelWithoutAServiceIsNotTakenDownByItsOwnInterruptionHandling(t *testing.T) {
	tunnelingShop(t)

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "tunnel"}, &bytes.Buffer{}, &stderr); code != exitOK {
		t.Fatalf("tunnel = %d\nstderr: %s", code, stderr.String())
	}
}
