package core

import (
	"regexp"
	"strings"
)

// Status is how an environment is doing right now, in one answer: what `hm status` prints and
// the MCP server's environment_status returns. Where describe says what a project is, this says
// whether it works — which is the question an agent asks between one change and the next.
type Status struct {
	Project  string `json:"project"`
	Root     string `json:"root"`
	Worktree string `json:"worktree"`

	// State is describe's: running, partial or stopped.
	State string `json:"state"`

	// Mode is Magento's deploy mode, and Xdebug "on", "off" or "unknown".
	Mode   string `json:"mode"`
	Xdebug string `json:"xdebug"`

	// Anonymised is "yes" or "unknown", never "no": unknown is never treated as safe, and the
	// state is cleared by everything that replaces the database.
	Anonymised   string `json:"anonymised"`
	AnonymisedAt string `json:"anonymised_at"`

	Services []ServiceStatus `json:"services"`
	URLs     []URLStatus     `json:"urls"`
}

// ServiceStatus is one service: the state its container is in and the last line of its log that
// reported an error, empty when the recent log holds none or the container is not running.
type ServiceStatus struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Running   bool   `json:"running"`
	LastError string `json:"last_error"`
}

// URLStatus is one address and whether something answered it. Any HTTP answer counts, a 500 or a
// redirect included: the question is whether the request reached a server, and the code says the
// rest.
type URLStatus struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Answers bool   `json:"answers"`
	Code    int    `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
}

// maxErrorLine is how much of an error line is kept: a stack trace on one line is kilobytes, and
// the line is a pointer to the log, not the log.
const maxErrorLine = 300

// errorWord is what the services' logs say when something failed: nginx's [error], MariaDB's
// [ERROR], PHP's Fatal error, Monolog's CRITICAL. Whole words only, so error_reporting and
// log_errors in a configuration dump are not errors; a warning is not one either.
var errorWord = regexp.MustCompile(`(?i)\b(error|fatal|critical|emergency|alert|panic)\b`)

// LastErrorLine is the last line of a log that reports an error, trimmed, or "" when none does.
func LastErrorLine(log string) string {
	lines := strings.Split(log, "\n")

	for at := len(lines) - 1; at >= 0; at-- {
		line := strings.TrimSpace(lines[at])
		if line == "" || !errorWord.MatchString(line) {
			continue
		}

		if runes := []rune(line); len(runes) > maxErrorLine {
			line = string(runes[:maxErrorLine]) + "…"
		}

		return line
	}

	return ""
}
