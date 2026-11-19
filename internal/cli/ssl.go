package cli

import (
	"io"
)

// ssl generates a certificate for the domain given, or the project's own, or localhost, and
// installs it in the TLS terminator — what ssl.sh did. One argument at most: the shell read the
// first and ignored the rest, which made a typo a certificate for the wrong name.
func ssl(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 1 {
		return failure(stderr, jsonOutput, "ssl", exitUsage, "invalid_argument",
			"ssl takes one domain at most", binaryName()+" ssl [domain]")
	}

	project, code := projectOr(stderr, jsonOutput, "ssl")
	if code != 0 {
		return code
	}

	engine := newEngine(stdout, stderr, jsonOutput)

	domain := ""
	if len(args) == 1 {
		domain = args[0]
	} else {
		domain = engine.Property(project, "DOMAIN")
	}

	if domain == "" {
		domain = "localhost"
	}

	return report(stderr, jsonOutput, "ssl", engine.InstallCertificate(here(), domain))
}
