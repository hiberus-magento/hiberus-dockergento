package core

import "fmt"

//
// Switching this tool's own installation between versions and branches. The installation is a git
// checkout, so a version is a tag and a branch is one of origin's.
//

// Versions is what the installation can be switched to, and what it is on now.
type Versions struct {
	// Current is the branch checked out, or the nearest tag when the checkout is detached, or
	// "unknown" when there is none.
	Current  string    `json:"current"`
	Versions []Version `json:"versions"`

	// Branches is origin's, without the "origin/" in front, which is how they are switched to.
	Branches []string `json:"branches"`
}

// Version is one tag that looks like a version.
type Version struct {
	Name string `json:"name"`

	// PreRelease is a tag with a dash in it — a release candidate is there to be tried on purpose,
	// not to be picked by mistake by somebody reading the list from the top.
	PreRelease bool `json:"pre_release"`
	Installed  bool `json:"installed"`
}

// Switched is where a switch left the installation: the reference asked for, and the version git
// describes it as.
type Switched struct {
	Reference string `json:"switched_to"`
	Version   string `json:"version"`
}

// DirtyInstallation is a switch refused because the installation has changes nobody committed.
//
// A refusal of its own and not only a Refusal, because a person is shown the changes themselves,
// while a program is shown the refusal: what is in the way is what decides what to do about it.
type DirtyInstallation struct {
	// Directory is the installation, and Changes what `git status --porcelain` said about it, one
	// change per line.
	Directory string
	Changes   string
}

func (d DirtyInstallation) Error() string { return d.Refusal().Message }

// Refusal is the dirty installation as every other refusal is reported.
func (d DirtyInstallation) Refusal() Refusal {
	return Refusal{
		Kind:    "dirty_installation",
		Code:    6,
		Message: "The installation directory has uncommitted changes, so the version was not changed",
		Hint:    fmt.Sprintf("Commit or stash them in %s", d.Directory),
	}
}

// Unwrap lets a caller that only knows Refusal still read the kind and the exit code.
func (d DirtyInstallation) Unwrap() error { return d.Refusal() }
