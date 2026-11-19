package app

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What tunnel.sh did, held by the use case: a service of the project forwarded to a free port of
// this machine, through a socat relay on the service's network, published on 127.0.0.1 only.
//

type services struct {
	names []string
	err   error
}

func (s services) Services(core.Project, core.ComposeFiles) ([]string, error) { return s.names, s.err }

type exposure map[string][]int

func (e exposure) Exposed(container string) ([]int, error) { return e[container], nil }

func tunneling(t *testing.T, tunnels *relays, exposed []int, reachableAfter int) Tunneler {
	t.Helper()

	tries := 0

	return Tunneler{
		Engine: &engine{containers: []core.Container{
			{ID: "db1", ComposeProject: "shop", ComposeService: "db", Running: true,
				Networks: []string{"shop_default"}},
			{ID: "redis1", ComposeProject: "shop", ComposeService: "redis", Running: false},
		}},
		Relays:   tunnels,
		Services: services{names: []string{"phpfpm", "db", "redis"}},
		Exposure: exposure{"db1": exposed},
		Binary:   "hm",
		FreePort: func() (int, error) { return 54321, nil },
		Reachable: func(port int) bool {
			tries++

			return port == 54321 && tries > reachableAfter
		},
		Sleep: func(time.Duration) {},
	}
}

func TestATunnelForwardsTheServicesOnlyPortToAFreeLocalOne(t *testing.T) {
	tunnels := &relays{}

	opened, err := tunneling(t, tunnels, []int{3306}, 2).Open(shop, core.ComposeFiles{}, "db", 0)
	if err != nil {
		t.Fatal(err)
	}

	want := core.Tunnel{Service: "db", Host: "127.0.0.1", Port: 54321, ContainerPort: 3306, Relay: "hm-tunnel-shop-db"}
	if opened != want {
		t.Errorf("opened = %+v, want %+v", opened, want)
	}

	// Whatever a closed window left behind for this service goes first
	if !reflect.DeepEqual(tunnels.removed, []string{"hm-tunnel-shop-db*"}) {
		t.Errorf("removed %v", tunnels.removed)
	}

	wantRelay := core.Relay{
		Name: "hm-tunnel-shop-db", Image: "alpine/socat", Network: "shop_default",
		Command:       []string{"tcp-listen:3306,fork,reuseaddr", "tcp-connect:db:3306"},
		HostIP:        "127.0.0.1",
		HostPort:      54321,
		ContainerPort: 3306,
		AutoRemove:    true,
	}
	if len(tunnels.started) != 1 || !reflect.DeepEqual(tunnels.started[0], wantRelay) {
		t.Errorf("started %+v, want %+v", tunnels.started, wantRelay)
	}
}

func TestAPortSaidIsThePortForwarded(t *testing.T) {
	tunnels := &relays{}

	opened, err := tunneling(t, tunnels, []int{80, 443}, 0).Open(shop, core.ComposeFiles{}, "db", 8080)
	if err != nil || opened.ContainerPort != 8080 {
		t.Fatalf("opened = %+v, %v, want 8080 whatever is exposed", opened, err)
	}
}

func TestATunnelIsRefusedBeforeAnythingStarts(t *testing.T) {
	cases := []struct {
		name    string
		service string
		exposed []int
		kind    string
		code    int
		hint    string
	}{
		{"no service", "", []int{3306}, "missing_service", 2, "hm tunnel db"},
		{"a service the project does not have", "nope", []int{3306}, "unknown_service", 5, "hm tunnel db phpfpm redis"},
		{"a service that is not running", "redis", []int{6379}, "service_not_running", 5, "hm start redis"},
		{"several exposed ports", "db", []int{33060, 3306}, "ambiguous_port", 2, "hm tunnel db 3306 33060"},
		{"no exposed port", "db", nil, "ambiguous_port", 2, "hm tunnel db "},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tunnels := &relays{}

			_, err := tunneling(t, tunnels, tt.exposed, 0).Open(shop, core.ComposeFiles{}, tt.service, 0)

			refusal := assertRefusal(t, err, tt.kind, tt.code)
			if refusal.Hint != tt.hint {
				t.Errorf("hint = %q, want %q", refusal.Hint, tt.hint)
			}

			if len(tunnels.started) != 0 || len(tunnels.removed) != 0 {
				t.Errorf("started=%v removed=%v, want nothing touched", tunnels.started, tunnels.removed)
			}
		})
	}
}

func TestTheMessageOfAnAmbiguousPortCountsThem(t *testing.T) {
	_, err := tunneling(t, &relays{}, []int{33060, 3306}, 0).Open(shop, core.ComposeFiles{}, "db", 0)

	refusal := assertRefusal(t, err, "ambiguous_port", 2)
	if refusal.Message != "'db' exposes 2 ports, so the one to forward has to be said" {
		t.Errorf("message = %q", refusal.Message)
	}
}

func TestARelayThatCannotStartIsRefused(t *testing.T) {
	tunnels := &relays{startErr: errors.New("pull denied")}

	_, err := tunneling(t, tunnels, []int{3306}, 0).Open(shop, core.ComposeFiles{}, "db", 0)

	refusal := assertRefusal(t, err, "tunnel_failed", 3)
	if refusal.Hint != "docker pull alpine/socat" {
		t.Errorf("hint = %q", refusal.Hint)
	}
}

func TestARelayNothingAnswersOnIsClosedAndRefused(t *testing.T) {
	tunnels := &relays{}

	_, err := tunneling(t, tunnels, []int{3306}, 1000).Open(shop, core.ComposeFiles{}, "db", 0)

	refusal := assertRefusal(t, err, "tunnel_not_ready", 1)
	if refusal.Message != "The tunnel was opened but nothing is answering on 127.0.0.1:54321" ||
		refusal.Hint != "docker logs hm-tunnel-shop-db" {
		t.Errorf("refusal = %+v", refusal)
	}

	if !reflect.DeepEqual(tunnels.removed, []string{"hm-tunnel-shop-db*", "hm-tunnel-shop-db"}) {
		t.Errorf("removed %v, want the relay closed after giving up", tunnels.removed)
	}
}

func TestClosingTunnelsTakesTheServicesOrEveryOne(t *testing.T) {
	cases := map[string]string{"db": "hm-tunnel-shop-db*", "": "hm-tunnel-shop*"}

	for service, want := range cases {
		tunnels := &relays{present: true}

		closed, err := tunneling(t, tunnels, nil, 0).Close(shop, service)
		if err != nil || !closed {
			t.Errorf("Close(%q) = %v, %v", service, closed, err)
		}

		if !reflect.DeepEqual(tunnels.removed, []string{want}) {
			t.Errorf("Close(%q) removed %v, want %s", service, tunnels.removed, want)
		}
	}
}
