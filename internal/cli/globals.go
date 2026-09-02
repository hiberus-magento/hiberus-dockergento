package cli

import "os"

//
// The flags that belong to the tool rather than to any one command.
//
// They are accepted before the command name and, for most commands, after it as well — which is
// what the shell implementation does, and the reason this exists: consuming them only after the
// command meant `hm --no-json describe` never reached the Go implementation at all. It fell
// through to the shell one, silently, and looked exactly like it had worked.
//

// transparent commands are the ones whose output is data or a child process's, so a flag after
// the command name belongs to that child. `hm composer show --format=json` is asking Composer for
// JSON, not asking us.
var transparent = map[string]bool{
	"exec": true, "bash": true, "magento": true, "composer": true, "npm": true,
	"n98-magerun": true, "grunt": true, "test-unit": true, "test-integration": true,
	"cloud": true, "cloud-login": true, "masquerade": true, "mysql": true, "mysqldump": true,
	"docker-compose": true, "copy-to-container": true, "copy-from-container": true, "logs": true,
}

// globals takes the tool's own flags out and returns what is left: the command and its arguments.
//
// The environment variables are set rather than returned because that is how the rest of the tool
// already reads them — the palette, the non-interactive guard — and because the shell
// implementation reads them too when a command falls through to it.
func globals(args []string) (format string, rest []string) {
	rest = make([]string, 0, len(args))
	seen, opaque := false, false

	for _, argument := range args {
		if seen && opaque {
			rest = append(rest, argument)

			continue
		}

		switch argument {
		case "--json":
			format = "json"
		case "--no-json":
			format = "text"
		case "--yes":
			os.Setenv("HM_NON_INTERACTIVE", "1") //nolint:errcheck
		case "--force":
			os.Setenv("HM_FORCE", "1") //nolint:errcheck
		case "--no-color":
			os.Setenv("HM_NO_COLOR", "1") //nolint:errcheck
		default:
			if argument == "" || argument[0] != '-' {
				if !seen {
					seen = true
					opaque = transparent[argument]
				}
			}

			rest = append(rest, argument)
		}
	}

	return format, rest
}

// wanted decides the output format: what was asked for wins, then what was already decided, and
// when nobody has decided, a terminal gets text and anything else gets JSON.
//
// The middle one matters for a command reached through the shell entry point: it parses the flags
// itself and exports the answer, so without reading it back `bin/run --no-json serve` would lose
// the format on the way across.
func wanted(format string, stdout any) bool {
	if format == "" {
		format = os.Getenv("HM_OUTPUT_FORMAT")
	}

	switch format {
	case "json":
		return true
	case "text":
		return false
	}

	return !isTerminal(stdout)
}
