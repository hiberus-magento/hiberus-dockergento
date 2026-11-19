package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// A dump path is checked before anything is decided, written or started: a wrong path used to be
// a warning, after which setup carried on and asked the question — so an automated bootstrap hung
// instead of failing. Ported from tests/unit/setup_options_test.sh, which tested the same rule on
// the shell implementation's parser.
//
// Properties, VCS and the rest are left nil on purpose: reaching any of them would panic, which is
// the proof that nothing past the check ran.
//

func TestAMissingDumpStopsSetupBeforeAnythingElse(t *testing.T) {
	legacy := &shell{}
	setup := Setup{FS: filesystem{present: map[string]bool{}}, Legacy: legacy, Binary: "hm"}

	err := setup.Run(t.TempDir(), core.SetupOptions{Dump: "/nowhere/dump.sql"}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "dump_not_found" || refusal.Code != 2 {
		t.Fatalf("Run with a missing dump = %v, want a dump_not_found refusal with code 2", err)
	}

	if !strings.Contains(refusal.Message, "/nowhere/dump.sql") {
		t.Errorf("message = %q, want it to say which file it was", refusal.Message)
	}

	if len(legacy.ran) != 0 {
		t.Errorf("ran %v, want nothing", legacy.ran)
	}
}

// TestAHomeRelativeDumpIsExpandedRatherThanRefused: the path is a value, so no shell expanded it.
func TestAHomeRelativeDumpIsExpandedRatherThanRefused(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	setup := Setup{FS: filesystem{present: map[string]bool{}}, Legacy: &shell{}, Binary: "hm"}

	err := setup.Run(t.TempDir(), core.SetupOptions{Dump: "~/dump.sql"}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "/home/someone/dump.sql") {
		t.Fatalf("Run with ~/dump.sql = %v, want the expanded path checked", err)
	}
}
