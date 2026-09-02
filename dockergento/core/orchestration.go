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
}
