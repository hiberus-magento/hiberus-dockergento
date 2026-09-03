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

	// FS is what tells a project that has anonymisation rules of its own from one that has not.
	FS ports.FS

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

// Dumper is the export a person asks for by name.
//
// Deliberately not the one a snapshot takes: that one is a copy to restore from, so it is
// consistent and carries the routines, the triggers and the events. This one is what somebody
// asked to hand to a colleague or load somewhere else, and it skips the triggers because a dump
// that recreates them fails to load as anybody but the user who defined them.
const Dumper = `dump=$(command -v mariadb-dump || command -v mysqldump); ` +
	`"$dump" --skip-triggers -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"`

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

// Shell runs a shell command inside the database container, which is how its own environment is
// read: the credentials are true there and nowhere else.
func (d Database) Shell(project core.Project, command string, out io.Writer) (int, error) {
	container, err := d.container(project)
	if err != nil {
		return 0, err
	}

	return d.Runner.Run(container, []string{"bash", "-c", command}, nil, out)
}

// Capture runs a shell command inside the database container and keeps its output apart from its
// complaints, which is what writing a copy to a file requires.
func (d Database) Capture(project core.Project, command string, out, errors io.Writer) (int, error) {
	container, err := d.container(project)
	if err != nil {
		return 0, err
	}

	return d.Runner.Capture(container, []string{"bash", "-c", command}, out, errors)
}

// Network is the one the database container is attached to, which is where anything that has to
// reach it has to be attached too.
func (d Database) Network(container string) (string, error) {
	containers, err := d.Engine.Containers()
	if err != nil {
		return "", err
	}

	for _, candidate := range containers {
		if candidate.ID == container && len(candidate.Networks) > 0 {
			return candidate.Networks[0], nil
		}
	}

	return "", fmt.Errorf("the database container is not on any network")
}

// Feed sends a stream into the database client, which is how a dump gets in.
//
// What the client says goes to the caller and is not swallowed: a dump that half applies says why
// on its error output, and a command that hid that would leave somebody with a database in an
// unknown state and no reason.
func (d Database) Feed(project core.Project, dump io.Reader, out io.Writer) (int, error) {
	container, err := d.container(project)
	if err != nil {
		return 0, err
	}

	return d.Runner.Feed(container, Client(), dump, out)
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
