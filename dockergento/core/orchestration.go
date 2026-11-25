package core

// LogOptions is what `hm logs` was asked for.
//
// Enumerated rather than passed through as text. The shell implementation forwarded whatever it
// was given to Compose and had to know, separately, which flags take a value — without that,
// `hm logs --tail 3` read the 3 as a service name and refused to run.
type LogOptions struct {
	Follow     bool
	Timestamps bool
	Tail       string
	Since      string
	Until      string
	Index      int
	NoColor    bool
	NoPrefix   bool
}

// ExecOptions is how something is run inside a service.
type ExecOptions struct {
	User string

	// Interactive and Tty are decided from the terminal, not assumed: `hm magento` in a script
	// has no terminal to allocate, and asking for one there is how a command that works by hand
	// fails in CI.
	Interactive bool
	Tty         bool

	Detach     bool
	Index      int
	Privileged bool
	Workdir    string

	// Environment is how a value reaches the command without going through a shell that would
	// have to quote it. A SQL query full of quotes and backticks passed as an argument is a
	// quoting bug waiting for the right query.
	Environment []string
}

// DownOptions is what removing an environment was asked to remove.
//
// Compose's own `down` takes exactly these, and so does this: a flag it does not understand is a
// usage error here rather than a complaint from somewhere deeper.
type DownOptions struct {
	// Volumes deletes the data with the containers. It is one letter on the command line and it
	// is the only thing here that cannot be rebuilt.
	Volumes bool

	// RemoveOrphans takes containers of this project that the file no longer describes.
	RemoveOrphans bool

	// Images is "all" or "local", or empty for none.
	Images string

	// Timeout is how long a container is given to stop before it is killed, in seconds. Nil means
	// Compose's own default.
	Timeout *int
}

// DownPreview is what a down would remove, worked out without removing anything: the project's
// containers, and with Volumes its volumes that are actually there, the database's among them.
type DownPreview struct {
	Containers []string `json:"containers"`
	Volumes    []string `json:"volumes"`
}

// RestorePreview is what a restore would replace: the database, named as the container names it,
// and the copy that would take its place.
type RestorePreview struct {
	Snapshot Snapshot `json:"snapshot"`
	Database string   `json:"database"`
}

// StopOptions is what stopping an environment was asked for.
type StopOptions struct {
	// Services names what to stop. Empty is the whole environment.
	Services []string

	// Snapshot saves the database before anything is stopped.
	Snapshot bool

	// Timeout is how long a container is given to stop before it is killed, in seconds. Nil means
	// Compose's own default, which is what a stop always waited before the option existed.
	Timeout *int
}

// RestartOptions is what restarting an environment was asked for. A restart is a stop and then a
// start, and Timeout is its stop's, as in StopOptions; there is no snapshot, as there never was.
type RestartOptions struct {
	Services []string
	Timeout  *int
}

// MachineStop is what a machine-wide stop actually did.
type MachineStop struct {
	// Total is every running container found, and Others is how many of them do not belong to
	// the project asking — the same distinction the shell implementation announced.
	Total  int
	Others int

	// Stopped is how many actually stopped, which is Total unless some of them refused.
	Stopped int
}
