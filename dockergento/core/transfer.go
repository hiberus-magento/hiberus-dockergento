package core

import "strings"

// LeftOutOfCopies reports whether an entry of a copied tree stays where it is, given its path
// relative to the tree's top (slash-separated): the project's repository, the .git directory or a
// worktree's .git file at that top, which no copy between this machine and the container carries.
//
// The container's copy of the code is not where the repository is kept. One that got there from
// an earlier copy is stale, and copying it back out writes old objects over the host's — git keeps
// them read-only, so the copy fails halfway, after vendor was already emptied for it.
//
// Only the top: a package Composer installed from source has its own .git under vendor, and that
// one is part of the dependency, which the copy exists to carry.
func LeftOutOfCopies(relative string) bool {
	first, _, _ := strings.Cut(strings.TrimPrefix(relative, "./"), "/")

	return first == ".git"
}
