package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// probing answers each URL from a table and records what was asked; a URL left out is refused.
type probing struct {
	mu     sync.Mutex
	codes  map[string]int
	asked  []string
	refuse error
}

func (p *probing) Answer(url string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.asked = append(p.asked, url)

	if code, ok := p.codes[url]; ok {
		return code, nil
	}

	return 0, p.refuse
}

func described() core.Description {
	description := core.Description{}
	description.Project.Name = "shop"
	description.Project.Root = "/code/shop"
	description.Project.Status = "partial"
	description.Project.URLs = core.URLs{
		Base:    "https://shop.local/",
		Admin:   "https://shop.local/admin_x",
		Mail:    "http://localhost:8025",
		Mailhog: "http://localhost:8025",
	}
	description.Magento.Mode = "developer"
	description.Tooling.Xdebug = "off"
	description.Data.Anonymised = "yes"
	description.Data.AnonymisedAt = "2026-11-25 10:00"
	description.Services = []core.DescribedService{
		{Name: "db", State: "running"},
		{Name: "nginx", State: "running"},
		{Name: "search", State: "exited"},
	}

	return description
}

func TestStatusIsTheDescriptionWithWhatIsWrongAddedToIt(t *testing.T) {
	probe := &probing{
		codes:  map[string]int{"https://shop.local/": 200, "https://shop.local/admin_x": 302},
		refuse: errors.New("connection refused"),
	}

	var (
		logsAsked []string
		asking    sync.Mutex
	)

	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return described(), nil },
		Logs: func(_ context.Context, service string) (string, error) {
			asking.Lock()
			logsAsked = append(logsAsked, service)
			asking.Unlock()

			if service == "nginx" {
				return "2026/11/25 [error] upstream timed out\n200 GET /\n", nil
			}

			return "ready for connections\n", nil
		},
		Probe: probe,
	}

	got, err := reporter.Status()
	if err != nil {
		t.Fatalf("Status() = %v", err)
	}

	want := core.Status{
		Project: "shop", Root: "/code/shop", State: "partial", Mode: "developer", Xdebug: "off",
		Anonymised: "yes", AnonymisedAt: "2026-11-25 10:00",
		Services: []core.ServiceStatus{
			{Name: "db", State: "running", Running: true},
			{Name: "nginx", State: "running", Running: true, LastError: "2026/11/25 [error] upstream timed out"},
			{Name: "search", State: "exited"},
		},
		URLs: []core.URLStatus{
			{Name: "base", URL: "https://shop.local/", Answers: true, Code: 200},
			{Name: "admin", URL: "https://shop.local/admin_x", Answers: true, Code: 302},
			{Name: "mail", URL: "http://localhost:8025", Error: "connection refused"},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Status() (-want +got):\n%s", diff)
	}

	// A stopped container has no log worth reading: only the running ones are asked
	sort.Strings(logsAsked)
	if diff := cmp.Diff([]string{"db", "nginx"}, logsAsked); diff != "" {
		t.Errorf("logs asked (-want +got):\n%s", diff)
	}

	// mailhog is mail's alias, the same address: asked once
	sort.Strings(probe.asked)
	if diff := cmp.Diff([]string{"http://localhost:8025", "https://shop.local/", "https://shop.local/admin_x"}, probe.asked); diff != "" {
		t.Errorf("URLs probed (-want +got):\n%s", diff)
	}
}

func TestALogThatCannotBeReadIsSaidAndTheRestStillAnswers(t *testing.T) {
	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return described(), nil },
		Logs:     func(context.Context, string) (string, error) { return "", errors.New("no such container") },
		Probe:    &probing{refuse: errors.New("refused")},
	}

	got, err := reporter.Status()
	if err != nil {
		t.Fatalf("Status() = %v", err)
	}

	if got.Services[0].LastError != "(the log could not be read: no such container)" {
		t.Fatalf("LastError = %q, want the reason said", got.Services[0].LastError)
	}
}

func TestStatusFailsWhereDescribeFails(t *testing.T) {
	refusal := core.Refusal{Kind: "project_not_configured", Code: 4, Message: "not a project"}

	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return core.Description{}, refusal },
		Logs: func(context.Context, string) (string, error) {
			t.Fatal("a log was read for a project that could not be described")

			return "", nil
		},
		Probe: &probing{},
	}

	if _, err := reporter.Status(); !errors.Is(err, refusal) {
		t.Fatalf("Status() = %v, want the refusal passed on", err)
	}
}

// TestTheLogsAreReadAllAtOnce: one round trip to Compose per running service, one after the other,
// is a status that takes as long as all of them together. Each read here waits until every other
// one has started, so read one at a time the first would wait out its timeout.
func TestTheLogsAreReadAllAtOnce(t *testing.T) {
	description := described()
	description.Services = []core.DescribedService{
		{Name: "db", State: "running"}, {Name: "nginx", State: "running"}, {Name: "phpfpm", State: "running"},
	}

	var started sync.WaitGroup

	started.Add(len(description.Services))

	everyone := make(chan struct{})
	go func() { started.Wait(); close(everyone) }()

	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return description, nil },
		Logs: func(ctx context.Context, service string) (string, error) {
			started.Done()

			select {
			case <-everyone:
				return service + ": [error] seen\n", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
		Probe:      &probing{refuse: errors.New("refused")},
		LogTimeout: 5 * time.Second,
	}

	got, err := reporter.Status()
	if err != nil {
		t.Fatalf("Status() = %v", err)
	}

	for _, service := range got.Services {
		if service.LastError != service.Name+": [error] seen" {
			t.Errorf("%s: LastError = %q, want its own log read while the others were being read", service.Name, service.LastError)
		}
	}
}

// TestALogThatDoesNotAnswerIsSaidAndHoldsNothingUp: a log read that hangs — a daemon under load, a
// container stuck stopping — is given a deadline, its context is cancelled, and status answers with
// the rest instead of waiting for it. Even a read that ignores its context is not waited for.
func TestALogThatDoesNotAnswerIsSaidAndHoldsNothingUp(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })

	var deadline atomic.Bool

	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return described(), nil },
		Logs: func(ctx context.Context, service string) (string, error) {
			if service == "db" {
				_, has := ctx.Deadline()
				deadline.Store(has)
				<-stuck

				return "", nil
			}

			return "[error] nginx's own\n", nil
		},
		Probe:      &probing{refuse: errors.New("refused")},
		LogTimeout: 50 * time.Millisecond,
	}

	answered := make(chan core.Status, 1)

	go func() {
		got, _ := reporter.Status()
		answered <- got
	}()

	var got core.Status

	select {
	case got = <-answered:
	case <-time.After(5 * time.Second):
		t.Fatal("Status() waited for a log that never answered")
	}

	if !deadline.Load() {
		t.Error("the log was read without a deadline on its context")
	}

	if db := got.Services[0].LastError; !strings.HasPrefix(db, "(the log could not be read:") || !strings.Contains(db, "50ms") {
		t.Errorf("db: LastError = %q, want the read said to have timed out, with how long it was given", db)
	}

	if nginx := got.Services[1].LastError; nginx != "[error] nginx's own" {
		t.Errorf("nginx: LastError = %q, want it read regardless", nginx)
	}
}
