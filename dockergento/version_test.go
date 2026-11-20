package dockergento

import (
	"testing"
	"testing/fstest"
)

// A stamped engine describes itself as that release and never as a checkout.
func TestAStampedEngineIsARelease(t *testing.T) {
	installed, _ := New(Options{Root: t.TempDir(), Version: "2.0.0", Assets: fstest.MapFS{}}).Installed()

	if !installed.Release || installed.Version != "2.0.0" || installed.Tag != "2.0.0" || installed.Commit != "" {
		t.Fatalf("Installed = %+v, want the 2.0.0 release", installed)
	}
}
