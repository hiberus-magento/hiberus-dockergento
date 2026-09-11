package app

import (
	"bytes"
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Capture runs a command inside an arbitrary service's container and hands back what it printed,
// keeping its complaints apart: a caller that wants to read stdout cannot afford a warning landing
// in the middle of it.
type Capture struct {
	Engine ports.ContainerEngine
	Runner ports.ContainerRunner

	// Binary is the name the tool was invoked as, for the sentence that says how to start the
	// service.
	Binary string
}

// Run resolves the service to this project's own container, runs the command inside it, and
// answers with what it printed on stdout. Its complaints go to errors, untouched. A non-zero
// status comes back as-is — it is not this call's failure to report, only the command's — so
// error is reserved for when the command could not be run at all: resolution failed, or the
// container transport itself did.
func (c Capture) Run(project core.Project, service string, command []string, errors io.Writer) (string, int, error) {
	container, err := resolveContainer(c.Engine, project, service, c.Binary)
	if err != nil {
		return "", 0, err
	}

	var out bytes.Buffer

	status, err := c.Runner.Capture(container, command, &out, errors)
	if err != nil {
		return "", 0, err
	}

	return out.String(), status, nil
}
