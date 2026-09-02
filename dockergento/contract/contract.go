// Package contract is the shape of every answer the tool gives, whichever door it came through.
//
// One definition, because there are three doors now — the terminal, HTTP, and MCP after it — and
// a document that is "the same shape" in three places is a document that stops being the same the
// first time one of them changes. A reader should not have to know which door an answer came
// through.
package contract

import (
	"errors"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The exit codes, which are a contract callers branch on: 2 is a usage error, 4 is not a project,
// 6 is a refusal on purpose. Flattening them into "something went wrong" would break every script
// and every agent that reads them.
const (
	ExitOK      = 0
	ExitError   = 1
	ExitUsage   = 2
	ExitDocker  = 3
	ExitProject = 4
	ExitService = 5
	ExitBlocked = 6
)

// SchemaVersion is what tells a reader that the tool changed from a tool that broke.
const SchemaVersion = 1

// Envelope wraps every answer.
//
// The field order is the order it is written in, and it is compared byte for byte against the
// shell implementation's, so it is not free to change.
type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	OK            bool   `json:"ok"`
	Data          any    `json:"data,omitempty"`
	Error         *Fault `json:"error,omitempty"`
}

// Fault is what went wrong, with the sentence that fixes it.
type Fault struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Success is an answer.
func Success(command string, data any) Envelope {
	return Envelope{SchemaVersion: SchemaVersion, Command: command, OK: true, Data: data}
}

// Failure is a refusal or an error, with the code that says which kind it was.
func Failure(command string, code int, kind, message, hint string) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion, Command: command, OK: false,
		Error: &Fault{Code: code, Type: kind, Message: message, Hint: hint},
	}
}

// FailureFrom turns whatever an operation returned into an answer.
//
// A refusal keeps its own reason and code — "this port is taken by another environment" is
// something the reader can act on, where a generic failure tells them only to try again.
func FailureFrom(command string, err error, fallback int, kind string) Envelope {
	var refusal core.Refusal
	if errors.As(err, &refusal) {
		return Failure(command, refusal.Code, refusal.Kind, refusal.Message, refusal.Hint)
	}

	return Failure(command, fallback, kind, err.Error(), "")
}
