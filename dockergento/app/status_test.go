package app

import (
	"errors"
	"sort"
	"sync"
	"testing"

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

	var logsAsked []string

	reporter := StatusReporter{
		Describe: func() (core.Description, error) { return described(), nil },
		Logs: func(service string) (string, error) {
			logsAsked = append(logsAsked, service)

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
		Logs:     func(string) (string, error) { return "", errors.New("no such container") },
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
		Logs: func(string) (string, error) {
			t.Fatal("a log was read for a project that could not be described")

			return "", nil
		},
		Probe: &probing{},
	}

	if _, err := reporter.Status(); !errors.Is(err, refusal) {
		t.Fatalf("Status() = %v, want the refusal passed on", err)
	}
}
