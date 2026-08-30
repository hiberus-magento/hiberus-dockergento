package core

// Refusal is an operation the tool declined to perform, with the reason and what to do about it.
//
// A typed error rather than a message, because the exit code is a contract callers branch on: 6
// is a refusal on purpose, 5 is a service that is not there, 3 is Docker. Flattening them into
// "something went wrong" would break every script and every agent that reads them.
type Refusal struct {
	Kind    string
	Code    int
	Message string
	Hint    string
}

func (r Refusal) Error() string { return r.Message }
