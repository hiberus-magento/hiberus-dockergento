package core

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What ai-doctor.sh asked of jq, asked of the registration: an entry found by either of its keys,
// the defaults it printed for what an entry did not say, and the state it concluded.
//

func TestARegistrationThatIsNotJSONIsRefused(t *testing.T) {
	if _, err := ReadAIRegistration([]byte(`{"skills": `)); !errors.Is(err, ErrAIRegistrationCorrupt) {
		t.Fatalf("err = %v, want ErrAIRegistrationCorrupt", err)
	}
}

func TestAnEntryIsFoundByItsAbsoluteKeyAndThenItsRelativeOne(t *testing.T) {
	registration, err := ReadAIRegistration([]byte(`{
		"skills": {
			"/p/.claude/skills/both": {"checksum": "abs"},
			".claude/skills/both": {"checksum": "rel"},
			".claude/skills/relative": {"checksum": "rel"},
			"/p/.claude/skills/null": null,
			".claude/skills/null": {"checksum": "fell-through"},
			".claude/skills/empty": {}
		},
		"agents": {}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		found bool
		sum   string
	}{
		{"both", true, "abs"},
		{"relative", true, "rel"},
		{"null", true, "fell-through"},
		{"empty", false, ""},
		{"absent", false, ""},
	}

	for _, tt := range cases {
		entry, found := registration.Entry("skills", "/p/.claude/skills/"+tt.name, ".claude/skills/"+tt.name)
		if found != tt.found || entry.Checksum != tt.sum {
			t.Errorf("%s = (%+v, %v), want checksum %q found %v", tt.name, entry, found, tt.sum, tt.found)
		}
	}

	if _, found := registration.Entry("agents", "/p/.claude/skills/both", ".claude/skills/both"); found {
		t.Error("an entry of skills was found among agents")
	}
}

func TestWhatAnEntryDoesNotSayIsADash(t *testing.T) {
	registration, err := ReadAIRegistration([]byte(`{"skills": {
		"bare": {"checksum": "c"},
		"blank": {"origin": "", "version": "", "installed": null},
		"full": {"origin": "hiberus", "version": "1.2.0", "installed": "2026-11-19 10:20:30"}
	}}`))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]AIRegistrationEntry{
		"bare":  {Checksum: "c", Origin: "-", Version: "-", Installed: "-"},
		"blank": {Origin: "-", Version: "-", Installed: "-"},
		"full":  {Origin: "hiberus", Version: "1.2.0", Installed: "2026-11-19"},
	}

	for key, expected := range want {
		entry, found := registration.Entry("skills", "/nowhere/"+key, key)
		if !found {
			t.Fatalf("%s not found", key)
		}

		if diff := cmp.Diff(expected, entry); diff != "" {
			t.Errorf("%s (-want +got):\n%s", key, diff)
		}
	}
}

func TestTheTrackedSkillsAreInKeyOrder(t *testing.T) {
	registration, err := ReadAIRegistration([]byte(`{"skills": {"b": {}, "B": {}, "a/x": {}}, "agents": {"z": {}}}`))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"B", "a/x", "b"}, registration.Tracked("skills")); diff != "" {
		t.Errorf("tracked (-want +got):\n%s", diff)
	}

	if got := (AIRegistration{}).Tracked("skills"); len(got) != 0 {
		t.Errorf("an empty registration tracks %v", got)
	}
}

func TestTheStateOfAResource(t *testing.T) {
	shipped := func(digest string) *string { return &digest }

	cases := []struct {
		name              string
		recorded, current string
		shipped           *string
		want              string
	}{
		{"as installed", "d", "d", nil, AIStateCurrent},
		{"never recorded", "", "d", nil, AIStateUnknown},
		{"edited", "d", "e", nil, AIStateModified},
		{"behind the tool", "d", "d", shipped("n"), AIStateOutdated},
		{"what the tool has", "d", "d", shipped("d"), AIStateCurrent},
		{"edited and behind", "d", "e", shipped("n"), AIStateModified},
		{"unrecorded and behind", "", "d", shipped("n"), AIStateUnknown},
	}

	for _, tt := range cases {
		if got := AIResourceState(tt.recorded, tt.current, tt.shipped); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestThePlatformsAProjectAsksFor(t *testing.T) {
	cases := map[string][]string{
		`{"platforms": ["claude", "cursor"]}`: {"claude", "cursor"},
		`{"platforms": ["two words"]}`:        {"two", "words"},
		`{}`:                                  nil,
		`not json`:                            nil,
	}

	for contents, want := range cases {
		if diff := cmp.Diff(want, ReadAIPlatforms([]byte(contents))); diff != "" {
			t.Errorf("%s (-want +got):\n%s", contents, diff)
		}
	}
}

func TestWhereAPlatformKeepsEachKind(t *testing.T) {
	catalog := ReadAIPlatformCatalog([]byte(`{"platforms": {
		"claude": {"skills_dir": ".claude/skills", "agents_dir": ".claude/agents"},
		"bare": {"skills_dir": ".bare/skills"}
	}}`))

	cases := []struct{ platform, kind, want string }{
		{"claude", "skills", ".claude/skills"},
		{"claude", "agents", ".claude/agents"},
		{"bare", "agents", ""},
		{"unknown", "skills", ""},
	}

	for _, tt := range cases {
		if got := catalog.Directory(tt.platform, tt.kind); got != tt.want {
			t.Errorf("%s %s = %q, want %q", tt.platform, tt.kind, got, tt.want)
		}
	}
}
