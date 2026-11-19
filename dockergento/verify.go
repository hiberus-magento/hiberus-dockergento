package dockergento

import (
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/dockerd"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Verify runs whatever code checks the project in dir has installed, in its php container.
//
// Asked in the order bin/run asked before verify.sh ran: Docker first, then the project, then —
// in the use case — the php container.
func (e *Engine) Verify(dir string, options core.VerifyOptions) (core.Verification, error) {
	if !(dockerd.Daemon{Timeout: e.options.Timeout}).Reachable() {
		return core.Verification{}, core.Refusal{
			Kind: "docker_unavailable", Code: contract.ExitDocker,
			Message: "Docker is not running", Hint: "Start Docker and try again",
		}
	}

	project, err := e.Resolve(dir)
	if err != nil || project.Name == "" {
		return core.Verification{}, core.Refusal{
			Kind: "project_not_configured", Code: contract.ExitProject,
			Message: "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid",
			Hint:    e.options.Binary + " setup",
		}
	}

	return app.Verifier{
		Engine:   dockerd.Engine{Timeout: e.options.Timeout},
		Runner:   dockerd.Runner{},
		Changes:  gitvcs.Git{},
		Progress: e.options.Progress,
		Binary:   e.options.Binary,
	}.Verify(project, options)
}
