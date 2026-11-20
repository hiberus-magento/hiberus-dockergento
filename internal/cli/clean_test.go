package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What clean decides before anything is deleted: whether to collect at all, and that what is
// collected is exactly what was surveyed. The deleting itself is app.Collector's, pinned in
// dockergento/app/collect_test.go.
//

func somethingToCollect() core.Collection {
	collection := core.Collection{
		Environments: []core.Abandoned{{Name: "hm-clean-dead", Root: "/lab/gone-root"}},
		Volumes:      []string{"hm-clean-dead_workspace"},
		Worktrees:    []core.AbandonedWorktree{{Name: "padre/rama-muerta", Project: "hm-clean-dead-rama-muerta", Path: "/lab/no-existe"}},
		Hosts:        []string{},
	}
	collection.Unattributable.Volumes = []string{"hm-clean-stray_workspace"}
	collection.Unattributable.Environments = []core.Abandoned{}

	return collection
}

func nothingToCollect() core.Collection {
	collection := core.Collection{
		Environments: []core.Abandoned{}, Volumes: []string{}, Worktrees: []core.AbandonedWorktree{}, Hosts: []string{},
	}
	collection.Unattributable.Volumes = []string{}
	collection.Unattributable.Environments = []core.Abandoned{}

	return collection
}

func methods(calls []call) []string {
	listed := []string{}

	for _, one := range calls {
		listed = append(listed, one.Method)
	}

	return listed
}

func TestCleanWithoutForceOnlyLooks(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)
	fake.outcomes = []outcome{{collection: somethingToCollect()}}

	var stdout, stderr bytes.Buffer

	if code := clean(nil, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("clean = %d, want exitOK; stderr = %q", code, stderr.String())
	}

	if diff := cmp.Diff([]string{"Survey"}, methods(fake.calls)); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	if !strings.Contains(stdout.String(), "Nothing was deleted") || !strings.Contains(stdout.String(), "clean --force") {
		t.Fatalf("stdout = %q, want it to say nothing was deleted and how to collect", stdout.String())
	}
}

func TestCleanWithForceCollectsExactlyWhatWasSurveyed(t *testing.T) {
	for _, this := range []struct {
		name  string
		args  []string
		force string
	}{
		{"--force", []string{"--force"}, ""},
		{"HM_FORCE=1", nil, "1"},
	} {
		t.Run(this.name, func(t *testing.T) {
			t.Setenv("HM_FORCE", this.force)

			fake := answering(t)
			fake.outcomes = []outcome{{collection: somethingToCollect()}}

			var stdout, stderr bytes.Buffer

			if code := clean(this.args, &stdout, &stderr, false); code != exitOK {
				t.Fatalf("clean = %d, want exitOK; stderr = %q", code, stderr.String())
			}

			collected := somethingToCollect()
			collected.Removed = true

			want := []call{{Method: "Survey"}, {Method: "Collect", Collection: collected}}
			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCleanJSONWithoutForcePrintsTheCollectionAndCollectsNothing(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)
	fake.outcomes = []outcome{{collection: somethingToCollect()}}

	var stdout, stderr bytes.Buffer

	if code := clean(nil, &stdout, &stderr, true); code != exitOK {
		t.Fatalf("clean --json = %d, want exitOK; stderr = %q", code, stderr.String())
	}

	if diff := cmp.Diff([]string{"Survey"}, methods(fake.calls)); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	var envelope struct {
		OK   bool            `json:"ok"`
		Data core.Collection `json:"data"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want one document", stdout.String(), err)
	}

	if diff := cmp.Diff(somethingToCollect(), envelope.Data); diff != "" || !envelope.OK {
		t.Fatalf("document ok=%v, data (-want +got):\n%s", envelope.OK, diff)
	}
}

func TestCleanJSONWithForceCollectsAndSaysRemoved(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)
	fake.outcomes = []outcome{{collection: somethingToCollect()}}

	var stdout, stderr bytes.Buffer

	if code := clean([]string{"--force"}, &stdout, &stderr, true); code != exitOK {
		t.Fatalf("clean --force --json = %d, want exitOK; stderr = %q", code, stderr.String())
	}

	if diff := cmp.Diff([]string{"Survey", "Collect"}, methods(fake.calls)); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	var envelope struct {
		Data struct {
			Removed bool `json:"removed"`
		} `json:"data"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want one document", stdout.String(), err)
	}

	if !envelope.Data.Removed {
		t.Fatalf("document = %s, want removed: true", stdout.String())
	}
}

func TestCleanWithNothingToCollectExitsZeroWithoutCollecting(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)
	fake.outcomes = []outcome{{collection: nothingToCollect()}}

	var stdout, stderr bytes.Buffer

	if code := clean([]string{"--force"}, &stdout, &stderr, false); code != exitOK {
		t.Fatalf("clean --force = %d, want exitOK; stderr = %q", code, stderr.String())
	}

	if diff := cmp.Diff([]string{"Survey"}, methods(fake.calls)); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestCleanRefusesAnUnknownOptionAskingNothing(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)

	var stdout, stderr bytes.Buffer

	if code := clean([]string{"--all"}, &stdout, &stderr, false); code != exitUsage {
		t.Fatalf("clean --all = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("asked of the engine = %v, want nothing", methods(fake.calls))
	}

	if !strings.Contains(stderr.String(), "--all") {
		t.Fatalf("stderr = %q, want it to name the option", stderr.String())
	}
}

func TestCleanReportsASurveyErrorAndCollectsNothing(t *testing.T) {
	t.Setenv("HM_FORCE", "")

	fake := answering(t)
	fake.outcomes = []outcome{{err: errors.New("cannot reach the daemon")}}

	var stdout, stderr bytes.Buffer

	if code := clean([]string{"--force"}, &stdout, &stderr, false); code == exitOK {
		t.Fatalf("clean --force = exitOK, want the survey's failure")
	}

	if diff := cmp.Diff([]string{"Survey"}, methods(fake.calls)); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	if !strings.Contains(stderr.String(), "cannot reach the daemon") {
		t.Fatalf("stderr = %q, want the reason", stderr.String())
	}
}

// Asked interactively, only a yes deletes: `printf 'n\n' | hm clean --force` deletes nothing.
func TestCleanInteractivelyCollectsOnlyOnYes(t *testing.T) {
	for _, this := range []struct {
		answer string
		want   []string
	}{
		{"n\n", []string{"Survey"}},
		{"\n", []string{"Survey"}},
		{"y\n", []string{"Survey", "Collect"}},
	} {
		t.Run(strings.TrimSpace(this.answer), func(t *testing.T) {
			t.Setenv("HM_FORCE", "")

			fake := answering(t)
			fake.outcomes = []outcome{{collection: somethingToCollect()}}
			t.Setenv("HM_NON_INTERACTIVE", "")

			var stdout, stderr bytes.Buffer

			code := -1

			askingWith(t, this.answer, func() {
				code = clean([]string{"--force"}, &stdout, &stderr, false)
			})

			if code != exitOK {
				t.Fatalf("clean --force = %d, want exitOK", code)
			}

			if diff := cmp.Diff(this.want, methods(fake.calls)); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// A pipe on stdout makes the output JSON, and that must not make the question go away:
// `printf 'n\n' | hm clean --force >/dev/null` deletes nothing. Only an explicit yes, or --yes,
// deletes, and the document says whether anything was.
func TestCleanJSONInteractivelyCollectsOnlyOnYes(t *testing.T) {
	for _, this := range []struct {
		answer  string
		want    []string
		removed bool
	}{
		{"n\n", []string{"Survey"}, false},
		{"", []string{"Survey"}, false},
		{"y\n", []string{"Survey", "Collect"}, true},
	} {
		t.Run(strings.TrimSpace(this.answer), func(t *testing.T) {
			t.Setenv("HM_FORCE", "")

			fake := answering(t)
			fake.outcomes = []outcome{{collection: somethingToCollect()}}
			t.Setenv("HM_NON_INTERACTIVE", "")

			var stdout, stderr bytes.Buffer

			code := -1

			askingWith(t, this.answer, func() {
				code = clean([]string{"--force"}, &stdout, &stderr, true)
			})

			if code != exitOK {
				t.Fatalf("clean --force --json = %d, want exitOK; stderr = %q", code, stderr.String())
			}

			if diff := cmp.Diff(this.want, methods(fake.calls)); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}

			var envelope struct {
				Data struct {
					Removed bool `json:"removed"`
				} `json:"data"`
			}

			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want one document and nothing else", stdout.String(), err)
			}

			if envelope.Data.Removed != this.removed {
				t.Fatalf("document = %s, want removed: %v", stdout.String(), this.removed)
			}
		})
	}
}
