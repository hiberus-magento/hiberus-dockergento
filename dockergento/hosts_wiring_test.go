package dockergento

import (
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/selfrun"
)

// TestSetHostWritesTheBaseURLThroughThisBinary: set-host writes Magento's base URL with
// `magento config:set`, and magento is this binary's — its shell twin is retired, so bin/run only
// names the binary and exits 2. Asked through the shell entry point, `hm set-host <domain>` failed
// at that step with config_failed. Through this binary, like setup's install steps, it reaches the
// command that answers.
func TestSetHostWritesTheBaseURLThroughThisBinary(t *testing.T) {
	engine := New(Options{Root: t.TempDir()})

	if _, ok := engine.hosts().Legacy.(selfrun.Runner); !ok {
		t.Fatalf("set-host runs its magento step through %T, want this binary (selfrun.Runner)",
			engine.hosts().Legacy)
	}
}
