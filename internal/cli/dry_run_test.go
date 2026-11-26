package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// --dry-run on the commands that destroy: what would go, and nothing gone. The proof that nothing
// changed is the fake's log — every call is recorded, so the log holding only reads is the log
// holding no Down, no Collect, no Exec.
//

// DownPreview and RestorePreview are reads, recorded like every other call. They answer with
// nothing: previewing wraps the fake to give them something to say.
func (f *fakeEngine) DownPreview(dir string, options core.DownOptions) (core.DownPreview, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "DownPreview", Dir: dir, DownOptions: options})

	return core.DownPreview{}, f.outcomeFor(number).err
}

func (f *fakeEngine) RestorePreview(dir, name string) (core.Project, core.RestorePreview, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "RestorePreview", Dir: dir, Key: name})

	return f.project, core.RestorePreview{}, f.outcomeFor(number).err
}

type previewing struct {
	*fakeEngine

	down    core.DownPreview
	restore core.RestorePreview
}

func (p previewing) DownPreview(dir string, options core.DownOptions) (core.DownPreview, error) {
	_, err := p.fakeEngine.DownPreview(dir, options)

	return p.down, err
}

func (p previewing) RestorePreview(dir, name string) (core.Project, core.RestorePreview, error) {
	project, _, err := p.fakeEngine.RestorePreview(dir, name)

	return project, p.restore, err
}

func previewed(t *testing.T) (*fakeEngine, *previewing) {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test"}
	t.Setenv("NO_COLOR", "1")

	wrapper := &previewing{fakeEngine: fake}
	newEngine = func(_, _ io.Writer, _ bool) commands { return wrapper }

	return fake, wrapper
}

type dryRunEnvelope struct {
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Data    struct {
		DryRun      bool                `json:"dry_run"`
		Project     string              `json:"project"`
		WouldRemove map[string][]string `json:"would_remove"`
		Snapshot    *core.Snapshot      `json:"snapshot"`
	} `json:"data"`
}

func runDry(t *testing.T, args ...string) (string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr); code != exitOK {
		t.Fatalf("hm %v = %d, want exitOK\nstderr: %s", args, code, stderr.String())
	}

	return stdout.String(), stderr.String()
}

func dryEnvelope(t *testing.T, stdout string) dryRunEnvelope {
	t.Helper()

	var envelope dryRunEnvelope

	decoder := json.NewDecoder(strings.NewReader(stdout))
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("stdout %q is not one envelope: %v", stdout, err)
	}

	if decoder.More() {
		t.Fatalf("stdout %q holds more than one document", stdout)
	}

	if !envelope.OK || !envelope.Data.DryRun {
		t.Fatalf("envelope = %+v, want ok and dry_run", envelope)
	}

	return envelope
}

func TestDownDryRunSaysWhatWouldGoAndRemovesNothing(t *testing.T) {
	preview := core.DownPreview{
		Containers: []string{"shop-db-1", "shop-phpfpm-1"},
		Volumes:    []string{"shop_dbdata"},
		Networks:   []string{"shop_back", "shop_default"},
	}

	t.Run("json", func(t *testing.T) {
		fake, wrapper := previewed(t)
		wrapper.down = preview
		cwd := here()

		stdout, _ := runDry(t, "down", "-v", "--dry-run", "--json")

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "DownPreview", Dir: cwd, DownOptions: core.DownOptions{Volumes: true}},
		}
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}

		envelope := dryEnvelope(t, stdout)

		wantRemoved := map[string][]string{
			"containers": {"shop-db-1", "shop-phpfpm-1"}, "volumes": {"shop_dbdata"},
			"networks": {"shop_back", "shop_default"}, "databases": {}, "files": {},
		}
		if diff := cmp.Diff(wantRemoved, envelope.Data.WouldRemove); diff != "" || envelope.Command != "down" ||
			envelope.Data.Project != "shop" {
			t.Fatalf("down --dry-run = %+v (-want +got would_remove):\n%s", envelope, diff)
		}
	})

	t.Run("text", func(t *testing.T) {
		_, wrapper := previewed(t)
		wrapper.down = preview

		stdout, _ := runDry(t, "down", "-v", "--dry-run", "--no-json")

		want := "\nhm down -v would remove, in 'shop':\n\n" +
			"  container  shop-db-1\n" +
			"  container  shop-phpfpm-1\n" +
			"  volume     shop_dbdata\n" +
			"  network    shop_back\n" +
			"  network    shop_default\n" +
			"\nNothing was removed.\n"
		if stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})
}

func TestRestoreDryRunNamesTheCopyAndTheDatabaseAndRestoresNothing(t *testing.T) {
	snapshot := core.Snapshot{Name: "before", Path: "/snapshots/shop/before.sql.gz", Size: "12M",
		TakenAt: "2026-11-25 10:00"}

	t.Run("json", func(t *testing.T) {
		fake, wrapper := previewed(t)
		wrapper.restore = core.RestorePreview{Snapshot: snapshot, Database: "magento"}
		cwd := here()

		stdout, _ := runDry(t, "db", "restore", "before", "--dry-run", "--json")

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "RestorePreview", Dir: cwd, Key: "before"},
		}
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}

		envelope := dryEnvelope(t, stdout)

		wantRemoved := map[string][]string{
			"containers": {}, "volumes": {}, "networks": {}, "databases": {"magento"}, "files": {},
		}
		if diff := cmp.Diff(wantRemoved, envelope.Data.WouldRemove); diff != "" || envelope.Command != "db" {
			t.Fatalf("db restore --dry-run = %+v (-want +got would_remove):\n%s", envelope, diff)
		}

		if envelope.Data.Snapshot == nil || envelope.Data.Snapshot.Name != "before" || envelope.Data.Snapshot.Size != "12M" {
			t.Fatalf("snapshot = %+v, want the copy that would be restored", envelope.Data.Snapshot)
		}
	})

	t.Run("text", func(t *testing.T) {
		_, wrapper := previewed(t)
		wrapper.restore = core.RestorePreview{Snapshot: snapshot, Database: "magento"}

		stdout, _ := runDry(t, "db", "restore", "--dry-run", "before", "--no-json")

		want := "\nhm db restore before would replace, in 'shop':\n\n" +
			"  database   magento\n" +
			"\n  with before (12M, taken 2026-11-25 10:00): everything in it since then would be lost.\n" +
			"\nNothing was restored.\n"
		if stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})
}

func TestPurgeDryRunListsWhatIsThereAndDeletesNothing(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		fake, _ := previewed(t)
		fake.outcomes = []outcome{{}, {captured: "generated/code\npub/static/frontend\n"}}
		cwd := here()

		stdout, _ := runDry(t, "purge", "--dry-run", "--json")

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Capture", Dir: "/code/shop", Service: phpService, Command: []string{"sh", "-c", purgeListing()}},
		}
		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}

		envelope := dryEnvelope(t, stdout)

		wantRemoved := map[string][]string{
			"containers": {}, "volumes": {}, "networks": {}, "databases": {}, "files": {"generated/code", "pub/static/frontend"},
		}
		if diff := cmp.Diff(wantRemoved, envelope.Data.WouldRemove); diff != "" || envelope.Command != "purge" {
			t.Fatalf("purge --dry-run = %+v (-want +got would_remove):\n%s", envelope, diff)
		}
	})

	t.Run("text, nothing there", func(t *testing.T) {
		fake, _ := previewed(t)
		fake.outcomes = []outcome{{}, {captured: ""}}

		stdout, _ := runDry(t, "purge", "--dry-run", "--no-json")

		if want := "\nhm purge would remove nothing in 'shop': there is nothing generated.\n"; stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})

	// The listing is what rm would take: the same patterns, every one of them
	t.Run("the listing asks about every pattern purge removes", func(t *testing.T) {
		for _, pattern := range generated {
			if !strings.Contains(purgeListing(), pattern) {
				t.Errorf("listing %q does not ask about %s", purgeListing(), pattern)
			}
		}
	})
}

// clean already previews without --force: --dry-run is that preview, and wins over --force.
func TestCleanDryRunIsThePreviewEvenWithForce(t *testing.T) {
	collection := core.Collection{
		Environments: []core.Abandoned{{Name: "gone", Root: "/code/gone"}},
		Volumes:      []string{"gone_dbdata"},
	}

	for _, args := range [][]string{
		{"clean", "--dry-run", "--json"},
		{"clean", "--force", "--dry-run", "--json"},
		{"--force", "clean", "--dry-run", "--no-json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fake, _ := previewed(t)
			t.Setenv("HM_FORCE", "")
			fake.outcomes = []outcome{{collection: collection}}

			stdout, _ := runDry(t, args...)

			if diff := cmp.Diff([]call{{Method: "Survey"}}, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}

			if !strings.Contains(stdout, "gone") {
				t.Fatalf("stdout = %q, want what would be collected", stdout)
			}
		})
	}
}

// Without --dry-run each command still does what it did: the destroying call is made.
func TestWithoutDryRunTheCommandsStillDestroy(t *testing.T) {
	cases := []struct {
		args   []string
		method string
	}{
		{[]string{"down", "-v", "--json"}, "Down"},
		{[]string{"purge", "--json"}, "Exec"},
		{[]string{"clean", "--force", "--json"}, "Collect"},
	}

	for _, tt := range cases {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			fake, _ := previewed(t)
			t.Setenv("HM_FORCE", "")
			fake.outcomes = []outcome{{collection: core.Collection{Volumes: []string{"gone_dbdata"}}}}

			runDry(t, tt.args...)

			for _, asked := range fake.calls {
				if asked.Method == tt.method {
					return
				}
			}

			t.Fatalf("hm %v asked %+v, want a %s", tt.args, fake.calls, tt.method)
		})
	}
}
