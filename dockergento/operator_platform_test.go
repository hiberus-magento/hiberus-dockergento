package dockergento

import (
	"testing"
	"testing/fstest"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// TestTheOperatorIsToldThePlatformTheWayItsChecksNameIt: the operator's checks compare against
// "mac" and "linux", which is what Platform() answers. Given runtime.GOOS instead, macOS arrived as
// "darwin" and the check that vendor is not bound from the host never ran there, on start or on
// rebuild.
func TestTheOperatorIsToldThePlatformTheWayItsChecksNameIt(t *testing.T) {
	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}})

	if got := engine.operator(core.Project{Name: "shop"}).Platform; got != Platform() {
		t.Fatalf("operator platform = %q, want %q", got, Platform())
	}
}
