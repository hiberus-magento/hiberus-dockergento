package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Replacing the personal data with data that looks like it.
//
// The anonymiser is a tool in an image of its own, attached to the network the database is on. It
// is deliberately not a service of the project: a container in the compose file is a container
// somebody has to remember to remove, and this one has one job that lasts a minute.
const anonymiser = "hiberusmagento/masquerade"

// Where a project keeps its own anonymisation rules, and where the tool expects to find them.
const (
	rulesInProject   = "config/docker/masquerade"
	rulesInContainer = "/app/masquerade"
)

// Anonymise runs it over the project's database.
func (d Importer) Anonymise(project core.Project) error {
	container, err := d.Database.Ready(project)
	if err != nil {
		return err
	}

	credentials, err := d.credentials(project)
	if err != nil {
		return err
	}

	network, err := d.Database.Network(container)
	if err != nil {
		return err
	}

	command := []string{
		"masquerade", "run",
		"--platform=magento2",
		"--database=" + credentials["MYSQL_DATABASE"],
		"--username=" + credentials["MYSQL_USER"],
		"--password=" + credentials["MYSQL_PASSWORD"],
		"--host=db",
		"--port=3306",
		"--driver=mysql",
		"--locale=es_ES",
	}

	binds := []core.Bind{}

	// A project with rules of its own uses them; one without uses whatever the image ships
	rules := filepath.Join(project.Root, rulesInProject)
	if d.Database.FS != nil && d.Database.FS.IsDir(rules) {
		binds = append(binds, core.Bind{Source: rules, Target: rulesInContainer})
		command = append(command, "--config="+rulesInContainer)
	}

	d.announce("Anonymising database in localhost...\n")

	status, err := d.OneOff.Run(anonymiser, command, network, binds, false)
	if err != nil {
		return err
	}

	if status != 0 {
		return core.Refusal{
			Kind:    "anonymisation_failed",
			Code:    1,
			Message: "The database was not anonymised",
			Hint:    d.Binary + " masquerade   # to try again",
		}
	}

	return d.State.Record(project.Name, time.Now().Format("2006-01-02 15:04"))
}

// credentials are read from the database container's own environment, which is where they are
// already true — carrying them in from outside is how they drift from what the container has.
func (d Importer) credentials(project core.Project) (map[string]string, error) {
	wanted := []string{"MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD"}
	credentials := map[string]string{}

	for _, name := range wanted {
		var value strings.Builder

		if _, err := d.Database.Shell(project,
			fmt.Sprintf(`printf %%s "$%s"`, name), &value); err != nil {
			return nil, err
		}

		credentials[name] = strings.TrimSpace(value.String())
	}

	return credentials, nil
}

func (d Importer) announce(message string) {
	if d.Announce != nil {
		d.Announce(message)
	}
}
