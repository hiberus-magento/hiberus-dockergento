package composelib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// RecentLog against a Docker daemon that is a stand-in on localhost: the question is what the log
// request asks the daemon, and only a daemon on the wire sees that. Nothing here reaches a real one.
//

// fakeDaemon answers the few requests a log read makes, for one service whose containers started
// at the times given, and records the query of every log request.
type fakeDaemon struct {
	mu      sync.Mutex
	started map[string]string // container id → State.StartedAt
	logs    []map[string]string
}

var apiPath = regexp.MustCompile(`^(/v[0-9.]+)?(/.*)$`)

func (d *fakeDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := apiPath.FindStringSubmatch(r.URL.Path)[2]

	w.Header().Set("Api-Version", "1.44")
	w.Header().Set("Content-Type", "application/json")

	labels := map[string]string{
		"com.docker.compose.project":     "shop",
		"com.docker.compose.service":     "nginx",
		"com.docker.compose.oneoff":      "False",
		"com.docker.compose.config-hash": "x",
	}

	switch {
	case path == "/_ping":
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "OK")
	case path == "/containers/json":
		var listed []map[string]any
		for id := range d.started {
			listed = append(listed, map[string]any{"Id": id, "Names": []string{"/shop-nginx-" + id}, "Labels": labels, "State": "running"})
		}

		json.NewEncoder(w).Encode(listed) //nolint:errcheck
	case strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/containers/"), "/json")

		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"Id": id, "Name": "/shop-nginx-" + id,
			"State":  map[string]any{"Running": true, "Status": "running", "StartedAt": d.started[id]},
			"Config": map[string]any{"Tty": true, "Labels": labels},
		})
	case strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/logs"):
		query := map[string]string{}
		for key := range r.URL.Query() {
			query[key] = r.URL.Query().Get(key)
		}

		d.mu.Lock()
		d.logs = append(d.logs, query)
		d.mu.Unlock()

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "[error] since it started\n")
	default:
		http.NotFound(w, r)
	}
}

// onFakeDaemon points the Docker client at daemon, with a configuration directory of its own so
// that nothing of this machine's — a context, credentials — is read.
func onFakeDaemon(t *testing.T, daemon *fakeDaemon) (core.Project, core.ComposeFiles) {
	t.Helper()

	server := httptest.NewServer(daemon)
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_API_VERSION", "")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"),
		[]byte("services:\n  nginx:\n    image: nginx:1.28\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return core.Project{Name: "shop", Root: root}, core.ComposeFiles{Load: []string{"docker-compose.yml"}}
}

// sinceOf is the value the Docker client sends for a time: seconds and nanoseconds since the epoch.
func sinceOf(t *testing.T, startedAt string) string {
	t.Helper()

	at, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("%d.%09d", at.Unix(), at.Nanosecond())
}

// TestARecentLogStartsWhenTheContainerStarted: what a container logged before it last started —
// the SIGTERM of the stop before, an error from September — is not how it is doing now. The log
// is asked from the container's own StartedAt, with the tail on top of that.
func TestARecentLogStartsWhenTheContainerStarted(t *testing.T) {
	daemon := &fakeDaemon{started: map[string]string{"a1": "2026-11-25T10:00:00.5Z"}}
	project, files := onFakeDaemon(t, daemon)

	log, err := Orchestrator{Stdout: os.Stderr}.RecentLog(context.Background(), project, files, "nginx", "200")
	if err != nil {
		t.Fatalf("RecentLog() = %v", err)
	}

	if log != "[error] since it started\n" {
		t.Errorf("log = %q, want what the daemon answered", log)
	}

	if len(daemon.logs) != 1 {
		t.Fatalf("log requests = %v, want one", daemon.logs)
	}

	asked := daemon.logs[0]
	if want := sinceOf(t, "2026-11-25T10:00:00.5Z"); asked["since"] != want || asked["tail"] != "200" {
		t.Fatalf("log request = %v, want since=%s and tail=200", asked, want)
	}
}

// TestARecentLogOfSeveralContainersStartsAtTheEarliest: a service scaled to two containers is read
// from when the first of them started, so that neither loses what it said since its own start.
func TestARecentLogOfSeveralContainersStartsAtTheEarliest(t *testing.T) {
	daemon := &fakeDaemon{started: map[string]string{
		"a1": "2026-11-25T10:05:00Z",
		"b2": "2026-11-25T09:00:00Z",
	}}
	project, files := onFakeDaemon(t, daemon)

	if _, err := (Orchestrator{Stdout: os.Stderr}).RecentLog(context.Background(), project, files, "nginx", "200"); err != nil {
		t.Fatalf("RecentLog() = %v", err)
	}

	want := sinceOf(t, "2026-11-25T09:00:00Z")
	for _, asked := range daemon.logs {
		if asked["since"] != want {
			t.Errorf("log request = %v, want since=%s", asked, want)
		}
	}
}

// TestARecentLogWithNoStartTimeIsTheTail: a container Docker gives no start time for is read as
// before, the tail alone, rather than not at all.
func TestARecentLogWithNoStartTimeIsTheTail(t *testing.T) {
	daemon := &fakeDaemon{started: map[string]string{"a1": "0001-01-01T00:00:00Z"}}
	project, files := onFakeDaemon(t, daemon)

	if _, err := (Orchestrator{Stdout: os.Stderr}).RecentLog(context.Background(), project, files, "nginx", "200"); err != nil {
		t.Fatalf("RecentLog() = %v", err)
	}

	if len(daemon.logs) != 1 || daemon.logs[0]["since"] != "" || daemon.logs[0]["tail"] != "200" {
		t.Fatalf("log requests = %v, want the tail and no since", daemon.logs)
	}
}
