package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What share.sh did, held by the use case: a public address through a Cloudflare quick tunnel, in a
// container of its own on the network of the project's web service.
//

type relays struct {
	removed []string
	present bool

	started  []core.Relay
	startErr error

	// logs is what Logs answers, one entry per call, the last one repeated
	logs  []string
	asked int

	waited []string
}

func (r *relays) Remove(name string, prefix bool) (bool, error) {
	if prefix {
		name += "*"
	}

	r.removed = append(r.removed, name)

	return r.present, nil
}

func (r *relays) Start(relay core.Relay) error {
	r.started = append(r.started, relay)

	return r.startErr
}

func (r *relays) Logs(string) (string, error) {
	if len(r.logs) == 0 {
		return "", nil
	}

	answer := r.logs[min(r.asked, len(r.logs)-1)]
	r.asked++

	return answer, nil
}

func (r *relays) Wait(name string) error {
	r.waited = append(r.waited, name)

	return nil
}

func sharing(t *testing.T, running bool, tunnels *relays) (Sharer, *[]time.Duration, *[]string) {
	t.Helper()

	var slept []time.Duration

	var said []string

	containers := []core.Container{
		{ID: "php1", ComposeProject: "shop", ComposeService: "phpfpm", Running: true},
		{ID: "varnish1", ComposeProject: "shop", ComposeService: "varnish", Running: running,
			Networks: []string{"shop_default", "zz_proxy"}},
	}

	return Sharer{
		Engine: &engine{containers: containers},
		Relays: tunnels,
		Binary: "hm",
		Say:    func(message string) { said = append(said, message) },
		Sleep:  func(d time.Duration) { slept = append(slept, d) },
	}, &slept, &said
}

var shop = core.Project{Name: "shop", Domain: "shop.test"}

func TestSharingAnswersWithTheAddressTheTunnelReports(t *testing.T) {
	tunnels := &relays{logs: []string{
		"starting\n",
		"INF Requesting new quick Tunnel\nINF |  https://brave-otter-12.trycloudflare.com  |\n",
	}}
	sharer, slept, said := sharing(t, true, tunnels)

	shared, err := sharer.Open(shop, "varnish", 6081, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := core.Share{URL: "https://brave-otter-12.trycloudflare.com", Project: "shop", Relay: "hm-share-shop"}
	if shared != want {
		t.Errorf("shared = %+v, want %+v", shared, want)
	}

	// A tunnel a closed window left behind goes first, then the new one on the web service's first
	// network, with the Host header rewritten to the domain nginx has a vhost for
	if !reflect.DeepEqual(tunnels.removed, []string{"hm-share-shop"}) {
		t.Errorf("removed %v, want the stale relay first", tunnels.removed)
	}

	wantRelay := core.Relay{
		Name: "hm-share-shop", Image: "cloudflare/cloudflared:latest", Network: "shop_default",
		Command: []string{"tunnel", "--no-autoupdate", "--url", "http://varnish:6081",
			"--http-host-header", "shop.test"},
	}
	if len(tunnels.started) != 1 || !reflect.DeepEqual(tunnels.started[0], wantRelay) {
		t.Errorf("started %+v, want %+v", tunnels.started, wantRelay)
	}

	// Asked again every two seconds until the address appears
	if !reflect.DeepEqual(*slept, []time.Duration{2 * time.Second}) {
		t.Errorf("slept %v, want one 2s wait between the two reads", *slept)
	}

	if !reflect.DeepEqual(*said, []string{"Opening a tunnel...\n"}) {
		t.Errorf("said %q", *said)
	}
}

func TestAProjectWithoutADomainIsSharedAsLocalhost(t *testing.T) {
	tunnels := &relays{logs: []string{"https://a-b.trycloudflare.com"}}
	sharer, _, _ := sharing(t, true, tunnels)

	if _, err := sharer.Open(core.Project{Name: "shop"}, "varnish", 6081, nil); err != nil {
		t.Fatal(err)
	}

	command := tunnels.started[0].Command
	if command[len(command)-1] != "localhost" {
		t.Errorf("host header = %q, want localhost", command[len(command)-1])
	}
}

func TestSharingAStoppedServiceIsRefusedBeforeAnythingIsAsked(t *testing.T) {
	tunnels := &relays{}
	sharer, _, _ := sharing(t, false, tunnels)

	asked := false

	_, err := sharer.Open(shop, "varnish", 6081, func(string) bool { asked = true; return true })

	assertRefusal(t, err, "service_not_running", 5)

	if asked || len(tunnels.started) != 0 || len(tunnels.removed) != 0 {
		t.Errorf("asked=%v started=%v removed=%v, want nothing done", asked, tunnels.started, tunnels.removed)
	}
}

func TestAnsweringNoSharesNothing(t *testing.T) {
	tunnels := &relays{}
	sharer, _, said := sharing(t, true, tunnels)

	var question string

	shared, err := sharer.Open(shop, "varnish", 6081, func(project string) bool { question = project; return false })
	if err != nil {
		t.Fatal(err)
	}

	if !shared.Declined || question != "shop" {
		t.Errorf("shared = %+v, asked about %q, want declined after asking about shop", shared, question)
	}

	if len(tunnels.started) != 0 || len(tunnels.removed) != 0 {
		t.Errorf("started=%v removed=%v, want nothing touched", tunnels.started, tunnels.removed)
	}

	if !reflect.DeepEqual(*said, []string{"Nothing was shared.\n"}) {
		t.Errorf("said %q", *said)
	}
}

func TestATunnelThatCannotStartIsRefused(t *testing.T) {
	tunnels := &relays{startErr: errors.New("no such image")}
	sharer, _, _ := sharing(t, true, tunnels)

	_, err := sharer.Open(shop, "varnish", 6081, nil)

	refusal := assertRefusal(t, err, "tunnel_failed", 3)
	if refusal.Hint != "docker pull cloudflare/cloudflared:latest" {
		t.Errorf("hint = %q", refusal.Hint)
	}
}

func TestATunnelThatNeverReportsAnAddressIsClosedAndRefused(t *testing.T) {
	tunnels := &relays{logs: []string{"still connecting\n"}}
	sharer, slept, _ := sharing(t, true, tunnels)

	_, err := sharer.Open(shop, "varnish", 6081, nil)

	refusal := assertRefusal(t, err, "no_address", 1)
	if refusal.Hint != "docker logs hm-share-shop" {
		t.Errorf("hint = %q", refusal.Hint)
	}

	// Sixty seconds, two at a time, as the shell waited
	if len(*slept) != 30 {
		t.Errorf("slept %d times, want 30", len(*slept))
	}

	if !reflect.DeepEqual(tunnels.removed, []string{"hm-share-shop", "hm-share-shop"}) {
		t.Errorf("removed %v, want the relay closed again after giving up", tunnels.removed)
	}
}

func TestClosingSaysWhetherAnythingWasShared(t *testing.T) {
	for _, present := range []bool{true, false} {
		tunnels := &relays{present: present}
		sharer, _, _ := sharing(t, true, tunnels)

		closed, err := sharer.Close(shop)
		if err != nil || closed != present {
			t.Errorf("Close with a relay present=%v = %v, %v", present, closed, err)
		}

		if !reflect.DeepEqual(tunnels.removed, []string{"hm-share-shop"}) {
			t.Errorf("removed %v, want exactly the project's relay", tunnels.removed)
		}
	}
}

func assertRefusal(t *testing.T, err error, kind string, code int) core.Refusal {
	t.Helper()

	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a %s refusal", err, kind)
	}

	if refusal.Kind != kind || refusal.Code != code {
		t.Fatalf("refusal = %s/%d (%s), want %s/%d", refusal.Kind, refusal.Code,
			strings.TrimSpace(refusal.Message), kind, code)
	}

	return refusal
}
