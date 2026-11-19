package core

import (
	"errors"
	"strings"
	"testing"
)

// TestTheFingerprintIsTheOneJqHashed: the expected digest is `jq -S -c` of these facts piped
// through md5 — the value the shell implementation wrote, and the doctor's own test pins too.
func TestTheFingerprintIsTheOneJqHashed(t *testing.T) {
	got := AgentContextFingerprint("shop.test", "2.4.7-p3", "https://shop.test/admin_9k2", "unknown", []Service{
		{Name: "db", Image: "hiberusmagento/mariadb:10.6"},
		{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"},
	})

	if got != "7a0329048d77eff6ce9873e3f8c8fef7" {
		t.Fatalf("fingerprint = %q, want the one the shell implementation wrote", got)
	}
}

// TestStringsAreEscapedAsJqEscapesThem: encoding/json would escape the HTML characters and U+2028,
// and jq does not — a fingerprint over the other bytes would never match the one written.
func TestStringsAreEscapedAsJqEscapesThem(t *testing.T) {
	if got, want := jqString("a&<b> \"\\\n\x01\x7f/é"), `"a&<b>`+" "+`\"\\\n\u0001\u007f/é"`; got != want {
		t.Fatalf("jqString = %s, want %s", got, want)
	}
}

func TestTheBlockSaysWhatTheDataIs(t *testing.T) {
	context := AgentContext{Binary: "hm"}
	context.Description.Project.Name = "shop"
	context.Description.Data.Anonymised = "yes"
	context.Description.Data.AnonymisedAt = "2026-11-02 10:15"

	block := context.Block()

	if !strings.HasPrefix(block, AgentContextBegin+"\n<!-- hm:version unknown hm:fingerprint ") {
		t.Errorf("block starts %q, want the marker, then the version and the fingerprint", block[:120])
	}

	if !strings.HasSuffix(block, "\n\n"+AgentContextEnd) {
		t.Errorf("block ends %q, want the closing marker and no newline after it", block[len(block)-40:])
	}

	if !strings.Contains(block, "Anonymised on 2026-11-02 10:15. It is safe to quote rows in your output.") {
		t.Errorf("block does not say when the data was anonymised:\n%s", block)
	}

	if !strings.Contains(block, "| Magento | unknown |\n| Mode | unknown |\n") {
		t.Errorf("block does not say unknown for what nobody knows:\n%s", block)
	}

	if strings.Contains(block, "| Storefront |") || strings.Contains(block, "| Admin |") {
		t.Errorf("block has addresses for a project with no domain:\n%s", block)
	}
}

// TestADatabaseNobodyAnonymisedIsCalledWhatItIs: real personal data, said in so many words, and a
// fingerprint that changes when it is anonymised, so that the context is regenerated then.
func TestADatabaseNobodyAnonymisedIsCalledWhatItIs(t *testing.T) {
	plain := AgentContext{Binary: "hm"}
	plain.Description.Project.Name = "shop"
	plain.Description.Data.Anonymised = "unknown"

	block := plain.Block()

	for _, want := range []string{"This database has not been anonymised.", "real personal data"} {
		if !strings.Contains(block, want) {
			t.Errorf("block does not say %q:\n%s", want, block)
		}
	}

	anonymised := plain
	anonymised.Description.Data.Anonymised = "yes"

	if plain.Fingerprint() == anonymised.Fingerprint() {
		t.Error("anonymising leaves the fingerprint as it was; the context would not be regenerated")
	}
}

// TestEverythingOutsideTheMarkersIsLeftAsItWas, byte for byte, including a last line with no
// newline.
func TestEverythingOutsideTheMarkersIsLeftAsItWas(t *testing.T) {
	existing := "mine\n" + AgentContextBegin + "\nold\n" + AgentContextEnd + "\nalso mine"

	placement, contents, err := PlaceAgentContext([]byte(existing), true, "NEW")
	if err != nil || placement != PlacementReplace {
		t.Fatalf("PlaceAgentContext = %v, %v, want a replacement", placement, err)
	}

	if got, want := string(contents), "mine\nNEW\nalso mine"; got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}

	if placement, contents, _ := PlaceAgentContext([]byte("no block"), true, "NEW"); placement != PlacementAppend ||
		string(contents) != "\nNEW\n" {
		t.Errorf("a file with no block got %v %q, want the block appended", placement, contents)
	}

	if placement, contents, _ := PlaceAgentContext(nil, false, "NEW"); placement != PlacementCreate ||
		string(contents) != "# Agent notes\n\nNEW\n" {
		t.Errorf("no file got %v %q, want one created", placement, contents)
	}
}

func TestABlockThatWasNeverClosedIsNotReplaced(t *testing.T) {
	for _, existing := range []string{
		"mine\n" + AgentContextBegin + "\nhalf\n",
		AgentContextEnd + "\n" + AgentContextBegin + "\n",
	} {
		if _, _, err := PlaceAgentContext([]byte(existing), true, "NEW"); !errors.Is(err, ErrAgentContextNotClosed) {
			t.Errorf("PlaceAgentContext(%q) = %v, want ErrAgentContextNotClosed", existing, err)
		}
	}
}

// TestTheMCPEntryIsMergedWhereItStands: the other servers stay, in their order, and an entry of
// this tool's own is replaced where it is rather than moved to the end.
func TestTheMCPEntryIsMergedWhereItStands(t *testing.T) {
	existing := `{"mcpServers": {"hm": {"command": "old"}, "other": {"command": "x", "n": 1.50}}}`

	contents, write := MergeAgentMCP([]byte(existing), true, "hm", "/opt/hm/bin/hm", "/code/shop")
	if !write {
		t.Fatal("MergeAgentMCP declined a document it can merge")
	}

	want := `{
  "mcpServers": {
    "hm": {
      "command": "/opt/hm/bin/hm",
      "args": [
        "mcp"
      ],
      "cwd": "/code/shop"
    },
    "other": {
      "command": "x",
      "n": 1.50
    }
  }
}
`
	if string(contents) != want {
		t.Errorf("contents =\n%s\nwant\n%s", contents, want)
	}

	if _, write := MergeAgentMCP([]byte(`{"mcpServers": "x"}`), true, "hm", "c", "d"); write {
		t.Error("servers that are not an object were written over, want the file left alone")
	}

	if contents, _ := MergeAgentMCP([]byte(`{"mcpServers": {`), true, "hm", "c", "d"); !strings.Contains(string(contents), `"cwd": "d"`) {
		t.Errorf("a file that cannot be parsed got %q, want it replaced with the entry", contents)
	}
}
