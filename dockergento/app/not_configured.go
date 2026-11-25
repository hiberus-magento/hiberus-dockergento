package app

import (
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// NotConfiguredMessage is the sentence of the project_not_configured refusal, which scripts and
// tests match verbatim.
const NotConfiguredMessage = "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid"

// NotConfigured is the refusal for a directory that is not a configured project, or whose Docker
// configuration cannot be read. The reason, when there is one, says which file and why; binary
// is the name the tool was run as, for the hint.
//
// One constructor for describe, agent-context and the commands that need a project, so the kind,
// the exit code and the sentence they share cannot drift apart.
func NotConfigured(reason, binary string) core.Refusal {
	message := NotConfiguredMessage
	if reason != "" {
		message += ": " + reason
	}

	return core.Refusal{Kind: "project_not_configured", Code: contract.ExitProject, Message: message, Hint: binary + " setup"}
}
