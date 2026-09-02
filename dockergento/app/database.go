package app

import (
	"fmt"
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Database is the project's database, reached through the container that holds it.
type Database struct {
	Engine ports.ContainerEngine
	Runner ports.ContainerRunner

	// Binary is the name the tool was invoked as, for the sentence that says how to start it.
	Binary string
}

// The service the database runs in, named once.
const databaseService = "db"

// How to reach the client, resolved inside the container.
//
// MariaDB 11 removed the legacy `mysql` name, and the images this tool ships span both sides of
// that: 10.2 has only `mysql`, 11 has only `mariadb`. Choosing inside the container is also what
// lets the root password and the database name be read from the container's own environment
// rather than carried in from outside.
const client = `client=$(command -v mariadb || command -v mysql); ` +
	`"$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"`

// Client is the command that opens the database client inside the container.
func Client() []string { return []string{"bash", "-c", client} }

// Ready reports the container holding the database, refusing when it is not running.
func (d Database) Ready(project core.Project) (string, error) { return d.container(project) }

// Query runs one statement and writes what the database answers.
//
// The statement travels in the environment rather than in the command, which is not a
// nicety: a query is full of quotes and backticks, and passing it as an argument through a shell
// is a quoting bug waiting for the right query.
func (d Database) Query(project core.Project, statement string, out io.Writer) (int, error) {
	container, err := d.container(project)
	if err != nil {
		return 0, err
	}

	return d.Runner.Run(container,
		[]string{"bash", "-c", client + ` -e "$QUERY"`},
		[]string{"QUERY=" + statement}, out)
}

// container is the one holding this project's database, and refusing when it is not running is
// the whole point: every other answer would be a connection error three layers down.
func (d Database) container(project core.Project) (string, error) {
	containers, err := d.Engine.Containers()
	if err != nil {
		return "", err
	}

	for _, candidate := range containers {
		if candidate.Key() != project.Name || candidate.ComposeService != databaseService {
			continue
		}

		if !candidate.Running {
			break
		}

		return candidate.ID, nil
	}

	return "", core.Refusal{
		Kind:    "service_not_running",
		Code:    5,
		Message: fmt.Sprintf("Service '%s' is not running", databaseService),
		Hint:    fmt.Sprintf("%s start %s", d.Binary, databaseService),
	}
}
