package core

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

//
// The digest `ai-doctor` compares, against what console/helpers/digest.sh's hm_digest_path printed
// for the same tree: digest.golden is that function's own output, run on testdata/digest. The
// tree was chosen for what the function's `find | LC_ALL=C sort` decides — a-b.txt before a/b.txt
// and B.txt before README, a hidden file included, a nested directory, a file with no trailing
// newline — and for the two answers that are not a directory: a file, and nothing at all.
//

func TestTheDigestIsTheShells(t *testing.T) {
	golden, err := os.Open("testdata/digest.golden")
	if err != nil {
		t.Fatal(err)
	}
	defer golden.Close()

	tree := os.DirFS("testdata/digest")
	scanner := bufio.NewScanner(golden)

	for scanner.Scan() {
		name, want, _ := strings.Cut(scanner.Text(), " ")

		if got := DigestPath(tree, name); got != want {
			t.Errorf("DigestPath(%q) = %q, want %q", name, got, want)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
