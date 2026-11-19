package dockergento

import (
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// TestVerifyWithoutDockerSaysSoFirst: as bin/run did before anything else, wherever it is run —
// outside a project too. The checks themselves are in app/verify_test.go.
func TestVerifyWithoutDockerSaysSoFirst(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")

	_, err := New(Options{}).Verify(t.TempDir(), core.VerifyOptions{})

	if refusal := refusalOf(t, err); refusal.Kind != "docker_unavailable" || refusal.Code != 3 ||
		refusal.Message != "Docker is not running" || refusal.Hint != "Start Docker and try again" {
		t.Fatalf("refusal = %+v", refusal)
	}
}
