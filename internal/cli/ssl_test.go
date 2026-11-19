package cli

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What ssl asks of the engine: the project resolved, the domain decided — the argument, else the
// project's DOMAIN, else localhost — and one InstallCertificate with it. What installing is, mkcert
// and hitch included, is app.Operator.InstallCertificate's, and is tested there.
//

func TestSslInstallsForTheDomainGiven(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.properties = map[string]string{"DOMAIN": "shop.local"}
	cwd := here()

	if code := Run([]string{"ssl", "other.test"}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("ssl other.test = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "InstallCertificate", Dir: cwd, Domain: "other.test"},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestSslFallsBackToTheProjectsDomain(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.properties = map[string]string{"DOMAIN": "shop.local"}
	cwd := here()

	if code := Run([]string{"ssl"}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("ssl = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Property", Dir: "/code/shop", Key: "DOMAIN"},
		{Method: "InstallCertificate", Dir: cwd, Domain: "shop.local"},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestSslFallsBackToLocalhost(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	if code := ssl(nil, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("ssl = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Property", Dir: "/code/shop", Key: "DOMAIN"},
		{Method: "InstallCertificate", Dir: cwd, Domain: "localhost"},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestSslWithTwoArgumentsIsAUsageErrorAndAsksNothing(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	if code := ssl([]string{"one.test", "two.test"}, io.Discard, io.Discard, false); code != exitUsage {
		t.Fatalf("ssl with two domains = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("asked of the engine = %v, want nothing", fake.calls)
	}
}

func TestSslAnswersWithTheRefusalsOwnCode(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "service_not_running",
		Code:    5,
		Message: "Service 'hitch' is not running",
		Hint:    "hm start hitch",
	}}}

	stderr := &strings.Builder{}

	if code := ssl([]string{"shop.local"}, io.Discard, stderr, true); code != 5 {
		t.Fatalf("a refused ssl = %d, want 5", code)
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Error.Type != "service_not_running" {
		t.Errorf("error type = %q, want service_not_running", envelope.Error.Type)
	}
}

func TestSslOutsideAProjectAsksNothingMore(t *testing.T) {
	fake := answering(t)
	cwd := here()

	if code := ssl(nil, io.Discard, io.Discard, false); code != exitProject {
		t.Fatalf("ssl outside a project = %d, want exitProject", code)
	}

	if diff := cmp.Diff([]call{{Method: "Resolve", Dir: cwd}}, fake.calls); diff != "" {
		t.Fatalf("asked of the engine outside a project (-want +got):\n%s", diff)
	}
}
