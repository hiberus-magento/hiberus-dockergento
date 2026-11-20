package core

import (
	"strconv"
	"strings"
)

// Installation is which build of the tool this is, as its own checkout describes it.
//
// It exists because of what a bug report needs: not "1.4.5", but 1.4.5 and eleven commits, on this
// branch, with uncommitted changes — which is the difference between a report somebody can act on
// and one that says the version somebody happened to have tagged.
type Installation struct {
	Version      string `json:"version"`
	Tag          string `json:"tag"`
	CommitsAhead int    `json:"commits_ahead"`
	Commit       string `json:"commit"`
	Branch       string `json:"branch"`
	Detached     bool   `json:"detached"`
	Dirty        bool   `json:"dirty"`
	Path         string `json:"path"`

	// Release is a binary built and stamped as a release: its version is the stamp, it has no
	// checkout to describe, and Path is the executable itself.
	Release bool `json:"release,omitempty"`
}

// ReleaseInstallation is the installation of a binary stamped with version at build time, which
// is installed at path. Nothing about it is asked of git: there is no checkout beside it.
func ReleaseInstallation(version, path string) Installation {
	return Installation{Version: version, Tag: version, Path: path, Release: true}
}

// Tooling is what is underneath: the daemon and the compose that drive the containers.
type Tooling struct {
	Docker         string `json:"version"`
	Compose        string `json:"compose"`
	ComposeCommand string `json:"compose_command"`
}

// CommitsAhead reads how far past the tag a description is, rather than asking git a second time.
//
// A description is `<tag>-<commits>-g<hash>` when there are any, and the tag alone when there are
// none.
func CommitsAhead(description string) int {
	at := strings.LastIndex(description, "-g")
	if at < 0 {
		return 0
	}

	rest := description[:at]

	dash := strings.LastIndex(rest, "-")
	if dash < 0 {
		return 0
	}

	ahead, err := strconv.Atoi(rest[dash+1:])
	if err != nil {
		return 0
	}

	return ahead
}
