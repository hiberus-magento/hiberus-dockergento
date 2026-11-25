package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
)

//
// The steps of update reach the terminal through the real engine. Only answers that replace
// nothing are run here, from a file:// fixture: the executable the real engine would replace is
// this test binary.
//

// releasesFixture points the real engine at a list of releases on disk holding only tag, and
// returns nothing to clean: the variable is the test's.
func releasesFixture(t *testing.T, tag string) {
	t.Helper()

	fixture := filepath.Join(t.TempDir(), "releases.json")
	listing := `[{"tag_name":"` + tag + `","prerelease":false,"draft":false,"assets":[]}]`

	if err := os.WriteFile(fixture, []byte(listing), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(dockergento.ReleasesURLVariable, "file://"+fixture)
}

func TestAnUpToDateUpdateOnATerminalShowsTheLookUpAndTheSameAnswer(t *testing.T) {
	onATerminal(t, false)
	stamped(t, "2.1.0")
	releasesFixture(t, "2.1.0")

	var stdout, stderr bytes.Buffer

	if code := update(nil, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("update = %d, stderr %q", code, stderr.String())
	}

	if want := "Hiberus Dockergento 2.1.0 is up to date\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}

	if !strings.Contains(stderr.String(), "Looking up the latest release") || len(screen(stderr.String())) != 0 {
		t.Fatalf("stderr = %q, want the look-up drawn and nothing left of it", stderr.String())
	}
}

// Off a terminal, and as JSON on one, stderr is what it always was: nothing.
func TestAnUpdateOffATerminalDrawsNothing(t *testing.T) {
	for _, c := range []struct {
		name     string
		terminal bool
		json     bool
	}{{"redirected", false, false}, {"json on a terminal", true, true}} {
		t.Run(c.name, func(t *testing.T) {
			onATerminal(t, false)
			copyTerminal = func(any) bool { return c.terminal }
			stamped(t, "2.1.0")
			releasesFixture(t, "2.1.0")

			var stdout, stderr bytes.Buffer

			if code := update(nil, &stdout, &stderr, c.json); code != exitOK {
				t.Fatalf("update = %d, stderr %q", code, stderr.String())
			}

			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want nothing", stderr.String())
			}
		})
	}
}

// A look-up that fails is drawn failed, and the error it always printed follows it.
func TestAFailedLookUpIsDrawnBeforeTheError(t *testing.T) {
	onATerminal(t, false)
	stamped(t, "2.1.0")
	t.Setenv(dockergento.ReleasesURLVariable, "file://"+filepath.Join(t.TempDir(), "missing.json"))

	var stderr bytes.Buffer

	if code := update(nil, &bytes.Buffer{}, &stderr, false); code == exitOK {
		t.Fatal("update succeeded without releases")
	}

	shown := screen(stderr.String())
	if len(shown) < 2 || shown[0] != "✗ Looking up the latest release" ||
		!strings.Contains(strings.Join(shown[1:], "\n"), "Could not read the releases") {
		t.Fatalf("a terminal shows:\n%s", strings.Join(shown, "\n"))
	}
}
