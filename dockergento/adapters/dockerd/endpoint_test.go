package dockerd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Where the daemon is, is not a detail. The SDK reads DOCKER_HOST and nothing else, while Colima,
// Docker Desktop and Rancher all leave that unset and keep the socket in the CLI's context store.
// Getting this wrong means reporting "Docker is not running" on a machine where it is running.

func contextStore(t *testing.T, name, host string) string {
	t.Helper()

	dir := t.TempDir()
	digest := sha256.Sum256([]byte(name))
	meta := filepath.Join(dir, "contexts", "meta", hex.EncodeToString(digest[:]))

	if err := os.MkdirAll(meta, 0o755); err != nil {
		t.Fatal(err)
	}

	document := `{"Name":"` + name + `","Endpoints":{"docker":{"Host":"` + host + `"}}}`
	if err := os.WriteFile(filepath.Join(meta, "meta.json"), []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}

	configuration := `{"currentContext":"` + name + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(configuration), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestTheEnvironmentWinsWhenItSaysSomething(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", contextStore(t, "colima", "unix:///home/colima.sock"))
	t.Setenv("DOCKER_HOST", "tcp://127.0.0.1:2375")

	if host := Endpoint(); host != "tcp://127.0.0.1:2375" {
		t.Fatalf("endpoint = %q, want the one the context names", host)
	}
}

func TestOtherwiseItComesFromTheCurrentContext(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONFIG", contextStore(t, "colima", "unix:///home/colima.sock"))

	if host := Endpoint(); host != "unix:///home/colima.sock" {
		t.Fatalf("endpoint = %q, want the one the context names", host)
	}
}

func TestAContextNamedByTheEnvironmentIsUsed(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONFIG", contextStore(t, "otro", "unix:///home/otro.sock"))
	t.Setenv("DOCKER_CONTEXT", "otro")

	if host := Endpoint(); host != "unix:///home/otro.sock" {
		t.Fatalf("endpoint = %q, want the one the context names", host)
	}
}

// The default context is the SDK's own default, so there is nothing to resolve and nothing to
// override.
func TestTheDefaultContextResolvesToNothing(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONFIG", contextStore(t, "default", "unix:///nunca.sock"))
	t.Setenv("DOCKER_CONTEXT", "default")

	if host := Endpoint(); host != "" {
		t.Fatalf("endpoint = %q, want none", host)
	}
}

func TestAMachineWithNoConfigurationResolvesToNothing(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	if host := Endpoint(); host != "" {
		t.Fatalf("endpoint = %q, want no invented socket", host)
	}
}

func TestAContextThatIsNotInTheStoreResolvesToNothing(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONFIG", contextStore(t, "colima", "unix:///home/colima.sock"))
	t.Setenv("DOCKER_CONTEXT", "no-existe")

	if host := Endpoint(); host != "" {
		t.Fatalf("endpoint = %q, want nothing invented", host)
	}
}
