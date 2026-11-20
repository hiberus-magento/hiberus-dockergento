package core

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// A document is written back the way `jq .` wrote it: keys where they were, empty containers on
// one line, text outside ASCII as it is, and only what jq escapes escaped.
func TestAIJSONIsWrittenTheWayJqWroteIt(t *testing.T) {
	value, err := ParseAIJSON([]byte(`{"z": 1.50, "a": {}, "m": [], "s": "Hyvä <&> \"q\"\t\u007f", "n": null, "b": false}`))
	if err != nil {
		t.Fatal(err)
	}

	object := value.(*AIObject)
	object.Set("a", "replaced in place")
	object.Set("new", true)

	want := `{
  "z": 1.50,
  "a": "replaced in place",
  "m": [],
  "s": "Hyvä <&> \"q\"\t\u007f",
  "n": null,
  "b": false,
  "new": true
}
`
	if diff := cmp.Diff(want, string(EncodeAIJSON(object))); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestAIJSONRefusesWhatJqWouldNotParse(t *testing.T) {
	for _, contents := range []string{"{bad", "not json", "[1,"} {
		if _, err := ParseAIJSON([]byte(contents)); err == nil {
			t.Errorf("ParseAIJSON(%q) parsed", contents)
		}
	}
}

func TestAIConfigurationHasTheScriptsShape(t *testing.T) {
	config := AIConfiguration("skills,,agents", "claude", "", nil, time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600)))

	want := `{
  "resources": [
    "skills",
    "agents"
  ],
  "platforms": [
    "claude"
  ],
  "types": [],
  "custom_repositories": [],
  "configured_at": "2026-01-02 02:04:05"
}
`
	if diff := cmp.Diff(want, string(EncodeAIJSON(config))); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// A type matches a skill named after it, or one starting with it and a dash.
func TestAISkillTypeMatchesByPrefix(t *testing.T) {
	cases := map[string]bool{"hyva": true, "hyva-checkout": true, "hyvacheckout": false, "magento-x": false}

	for name, want := range cases {
		if got := AISkillTypeMatches(name, []string{"hyva", "php"}); got != want {
			t.Errorf("AISkillTypeMatches(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestAIRepositoryNameIsTheURLsBasename(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/acme/tools.git": "tools",
		"https://github.com/acme/tools":     "tools",
		"https://github.com/acme/tools/":    "tools",
	} {
		if got := AIRepositoryName(url); got != want {
			t.Errorf("AIRepositoryName(%q) = %q, want %q", url, got, want)
		}
	}
}
