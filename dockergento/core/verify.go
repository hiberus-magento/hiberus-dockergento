package core

//
// Checking code with whatever the project has installed. The tools differ from one project to the
// next, so a check whose tool is missing is skipped, never failed: "this project has no PHPStan"
// and "PHPStan failed" lead to opposite actions.
//

// VerifyOptions is what a verification is asked to look at.
type VerifyOptions struct {
	// Changed looks only at the PHP files the branch changed since it left the base branch.
	Changed bool

	// All adds the slow ones: the unit tests and the DI compilation.
	All bool
}

// The three things a check can end as.
const (
	CheckOK      = "ok"
	CheckFailed  = "failed"
	CheckSkipped = "skipped"
)

// Check is one tool's verdict. Output is what the tool said when it failed, or why it was skipped.
type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Problems int    `json:"problems"`
	Output   string `json:"output"`
}

// Verification is every check, in the order they ran.
type Verification struct {
	// Project is the compose project verified, for the heading a person reads.
	Project string `json:"-"`

	// Scope says what was looked at: "everything", "changed", or everything with the reason why
	// not only the changes.
	Scope  string  `json:"scope"`
	Failed int     `json:"failed"`
	Checks []Check `json:"checks"`
}

// Ran is how many checks were not skipped.
func (v Verification) Ran() int {
	ran := 0

	for _, check := range v.Checks {
		if check.Status != CheckSkipped {
			ran++
		}
	}

	return ran
}
