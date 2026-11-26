package core

import "testing"

func TestOnlyTheRepositoryAtTheTopIsLeftOutOfCopies(t *testing.T) {
	cases := []struct {
		relative string
		left     bool
	}{
		{".git", true},
		{".git/objects/00/1c2e", true},
		{"./.git/HEAD", true},
		{".", false},
		{"", false},
		{".gitignore", false},
		{".github/workflows/ci.yml", false},
		{"composer.json", false},
		{"vendor/acme/pkg/.git/HEAD", false},
	}

	for _, tt := range cases {
		t.Run(tt.relative, func(t *testing.T) {
			if got := LeftOutOfCopies(tt.relative); got != tt.left {
				t.Fatalf("LeftOutOfCopies(%q) = %v, want %v", tt.relative, got, tt.left)
			}
		})
	}
}
