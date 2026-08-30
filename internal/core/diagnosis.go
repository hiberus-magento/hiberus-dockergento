package core

import (
	"strconv"
	"strings"
)

// Severity is how much a finding matters: whether it stops the environment working, whether it
// will bite later, or whether it is simply how things stand.
const (
	SeverityOK      = "ok"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Finding is one thing the diagnosis has to say.
//
// The action is the part that makes it worth printing. "Ports 80 and 443 are taken" is a fact;
// "cd into that project and run hm stop" is the sentence somebody can act on without asking
// anybody, which is the whole reason this command exists.
type Finding struct {
	ID       string `json:"id"`
	Scope    string `json:"scope"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

// Diagnosis is every finding plus the count somebody reads first.
type Diagnosis struct {
	Checks  []Finding `json:"checks"`
	Summary struct {
		Total    int `json:"total"`
		OK       int `json:"ok"`
		Warnings int `json:"warnings"`
		Errors   int `json:"errors"`
	} `json:"summary"`
}

// Summarise counts what the findings say.
func (d *Diagnosis) Summarise() {
	d.Summary.Total = len(d.Checks)
	d.Summary.OK = 0
	d.Summary.Warnings = 0
	d.Summary.Errors = 0

	for _, finding := range d.Checks {
		switch finding.Severity {
		case SeverityOK:
			d.Summary.OK++
		case SeverityWarning:
			d.Summary.Warnings++
		case SeverityError:
			d.Summary.Errors++
		}
	}
}

// The sizes the memory verdict is decided against, kept as names rather than as numbers buried in
// a comparison.
const (
	gigabyte = 1024 * 1024 * 1024

	// Under this not even one project with its full stack is comfortable.
	minimumVMMemory = 4 * gigabyte

	// A machine with less than this has nothing to spare, so a small VM on it is the hardware
	// and not a setting somebody forgot.
	roomyHostMemory = 16 * gigabyte

	// What one environment on the agent profile measured at — php, nginx, database, search and
	// Redis — of which the search engine is 85%.
	memoryPerEnvironment = 550

	// Left for everything else that runs in there.
	memoryOverhead = 1024
)

// VMMemoryVerdict says whether the memory the containers actually have is enough, and enough
// compared to what the machine has.
//
// The two numbers are different on macOS and identical on Linux, and that difference is the whole
// reason this exists: a laptop with 48 GB whose Docker VM has 6 is a laptop that fits six
// environments, and nothing said so. The symptom was environments that would not start, on a
// machine with plenty of memory free.
func VMMemoryVerdict(vm, host int64) string {
	if vm <= 0 {
		return "unknown"
	}

	if vm < minimumVMMemory {
		return "small"
	}

	if host >= roomyHostMemory && vm < host/4 {
		return "cramped"
	}

	return "fine"
}

// EnvironmentsThatFit is how many environments the given memory holds at once.
func EnvironmentsThatFit(vm int64) int {
	fit := (vm/1024/1024 - memoryOverhead) / memoryPerEnvironment
	if fit < 0 {
		return 0
	}

	return int(fit)
}

// Gigabytes rounds rather than truncates: 6.2 GB reported as "5" is the kind of small lie that
// makes somebody check the number somewhere else and stop trusting the rest.
func Gigabytes(bytes int64) int64 {
	return (bytes + gigabyte/2) / gigabyte
}

// VersionAtLeast compares two dotted versions the way `sort -V` does for the shapes this tool
// ever sees. A version that cannot be read at all is not "older": it is unknown, and the caller
// has already dealt with the empty case.
func VersionAtLeast(version, target string) bool {
	left, right := numbers(version), numbers(target)

	for at := 0; at < len(left) || at < len(right); at++ {
		one, other := 0, 0

		if at < len(left) {
			one = left[at]
		}

		if at < len(right) {
			other = right[at]
		}

		if one != other {
			return one > other
		}
	}

	return true
}

func numbers(version string) []int {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".")
	values := make([]int, 0, len(parts))

	for _, part := range parts {
		digits := part
		for at, character := range part {
			if character < '0' || character > '9' {
				digits = part[:at]

				break
			}
		}

		value, err := strconv.Atoi(digits)
		if err != nil {
			value = 0
		}

		values = append(values, value)
	}

	return values
}
