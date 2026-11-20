package core

//
// Moving this tool between its releases. The tool is one self-contained binary, so a version is a
// release published on GitHub with a binary for each platform, and moving to it is replacing the
// running executable with that binary.
//

// Versions is what the tool can be switched to, and what is running now.
type Versions struct {
	// Current is the version of the running binary, or "devel" for a binary built from a checkout.
	Current  string    `json:"current"`
	Versions []Version `json:"versions"`
}

// Version is one release of 2.0 or later.
type Version struct {
	Name string `json:"name"`

	// PreRelease is a release GitHub marks as one — a release candidate is there to be tried on
	// purpose, not to be picked by mistake by somebody reading the list from the top.
	PreRelease bool `json:"pre_release"`
	Installed  bool `json:"installed"`
}

// SwitchOptions is what `hm switch` was asked for: a version, or the newest final release.
type SwitchOptions struct {
	// Running is the version of the binary doing the switch.
	Running string
	Version string
	Stable  bool
}

// Switched is where a switch left the tool: the version asked for, the version now installed, the
// one it replaced and the executable that was replaced.
type Switched struct {
	Reference string `json:"switched_to"`
	Version   string `json:"version"`
	From      string `json:"from"`
	Path      string `json:"path"`
	// Unchanged is a switch to the version already running: nothing was downloaded or replaced.
	Unchanged bool `json:"unchanged"`
}

// UpdateOptions is what `hm update` was asked for.
type UpdateOptions struct {
	// Running is the version of the binary doing the update.
	Running string
	// PreReleases lets the update move to a pre-release when it is the newest release.
	PreReleases bool
}

// Updated is what an update did: whether it replaced the executable, from which version to which.
// Latest is the newest release it considered, empty when none of 2.0 or later is published.
type Updated struct {
	Updated bool   `json:"updated"`
	From    string `json:"from"`
	To      string `json:"to"`
	Latest  string `json:"latest"`
	Path    string `json:"path"`
}
