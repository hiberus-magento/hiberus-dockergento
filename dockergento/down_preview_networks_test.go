package dockergento

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The networks down --dry-run lists: the ones the project's Compose files declare as its own, that
// Docker actually has. Asked of a stand-in daemon on localhost (DOCKER_HOST=tcp://), so what the
// engine wires together — Compose's reading of the files and Docker's answer — is proved on the
// wire, with no daemon of the machine's.
//

// fakeDaemon answers a ping, an empty list of containers, and a network inspection for the names
// given; anything else is not found. A dry run that asked Docker to change something fails.
func fakeDaemon(t *testing.T, networks ...string) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			t.Errorf("%s %s: a dry run changes nothing", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")

		switch path := r.URL.Path; {
		case strings.HasSuffix(path, "/_ping"):
			w.Header().Set("Api-Version", "1.44")
			io.WriteString(w, "OK") //nolint:errcheck
		case strings.HasSuffix(path, "/containers/json"):
			io.WriteString(w, "[]") //nolint:errcheck
		case strings.Contains(path, "/networks/"):
			name := path[strings.LastIndex(path, "/")+1:]
			for _, present := range networks {
				if name == present {
					io.WriteString(w, `{"Name": "`+name+`", "Id": "`+name+`"}`) //nolint:errcheck

					return
				}
			}

			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"message": "network `+name+` not found"}`) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"message": "not found"}`) //nolint:errcheck
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
}

// shopProject is a project called tienda whose docker-compose.yml is the one given, in a HOME of
// its own, and an engine that knows nothing of this machine.
func shopProject(t *testing.T, compose string) (*Engine, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	files := map[string]string{
		"docker-compose.yml":            compose,
		"config/docker/properties.json": `{"COMPOSE_PROJECT_NAME": "tienda"}` + "\n",
	}

	for name, contents := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	engine := New(Options{
		Root:        t.TempDir(),
		Assets:      fstest.MapFS{},
		StateDir:    filepath.Join(home, "state"),
		WorktreeDir: filepath.Join(home, "worktrees"),
	})

	return engine, dir
}

func TestDownPreviewListsTheDeclaredNetworksDockerHas(t *testing.T) {
	engine, dir := shopProject(t, `services:
  phpfpm:
    image: alpine:latest
    networks: [default, front, proxy]
networks:
  front: {}
  proxy:
    external: true
    name: hm-proxy
`)

	// tienda_default is declared and gone; hm-proxy is there and is not the project's
	fakeDaemon(t, "tienda_front", "hm-proxy")

	preview, err := engine.DownPreview(dir, core.DownOptions{})
	if err != nil {
		t.Fatalf("DownPreview() error = %v", err)
	}

	if diff := cmp.Diff([]string{"tienda_front"}, preview.Networks); diff != "" {
		t.Errorf("networks (-want +got):\n%s", diff)
	}
}

// TestDownPreviewFailsWhenTheNetworksCannotBeRead: Compose files that cannot be read are no
// answer about the networks, and a preview that listed none would read as though there were none.
func TestDownPreviewFailsWhenTheNetworksCannotBeRead(t *testing.T) {
	engine, dir := shopProject(t, "services:\n  phpfpm: [\n")
	fakeDaemon(t, "tienda_default")

	preview, err := engine.DownPreview(dir, core.DownOptions{})
	if err == nil {
		t.Fatalf("DownPreview() = %+v, want the error reading the Compose files", preview)
	}

	if !strings.Contains(err.Error(), "networks") {
		t.Errorf("error = %q, want it to say it was the networks that could not be read", err)
	}
}
