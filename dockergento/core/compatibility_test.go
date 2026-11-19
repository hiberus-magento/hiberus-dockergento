package core

import (
	"os"
	"testing"
)

//
// The table `compatibility` prints, against what compatibility.sh printed for the same data: the
// fixture's table.txt is that script's own output, run on the two JSON files beside it. The data
// was chosen for the orderings the script got from `sort -V` and from jq's sort_by — 2.4.10 after
// 2.4.2 among the buckets, a release before its patches, -p10 after -p2 — and for a bucket of
// another line (2.5) the table leaves out.
//

func TestTheCompatibilityTableIsTheShells(t *testing.T) {
	read := func(name string) string {
		contents, err := os.ReadFile("testdata/compatibility/" + name)
		if err != nil {
			t.Fatal(err)
		}

		return string(contents)
	}

	requirements, err := ReadRequirements(read("requirements.json"))
	if err != nil {
		t.Fatal(err)
	}

	equivalents, err := ReadEquivalents(read("equivalent_versions.json"))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := CompatibilityTable(requirements, equivalents), read("table.txt"); got != want {
		t.Errorf("table:\n%s\nwant:\n%s", got, want)
	}
}
