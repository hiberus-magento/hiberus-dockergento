package app

import (
	"sync"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// URLProbe asks whether something answers at an address: the HTTP code when it does, an error
// when nothing did. Short and with a timeout, because a status that hangs on a dead domain is a
// status nobody runs twice.
type URLProbe interface {
	Answer(url string) (int, error)
}

// StatusReporter answers how an environment is doing: describe's answer, which already has the
// services, the URLs, the mode, xdebug and the anonymisation, with the two things it does not
// ask — whether the URLs answer, and the last error each running service logged.
type StatusReporter struct {
	// Describe is the project's description, the one `hm describe` prints
	Describe func() (core.Description, error)

	// Logs is the recent log of one service
	Logs func(service string) (string, error)

	Probe URLProbe
}

// Status asks the description once, then the logs and the URLs, the URLs all at once.
func (r StatusReporter) Status() (core.Status, error) {
	description, err := r.Describe()
	if err != nil {
		return core.Status{}, err
	}

	status := core.Status{
		Project:      description.Project.Name,
		Root:         description.Project.Root,
		Worktree:     description.Project.Worktree,
		State:        description.Project.Status,
		Mode:         description.Magento.Mode,
		Xdebug:       description.Tooling.Xdebug,
		Anonymised:   description.Data.Anonymised,
		AnonymisedAt: description.Data.AnonymisedAt,
		Services:     []core.ServiceStatus{},
		URLs:         []core.URLStatus{},
	}

	var probing sync.WaitGroup

	// mailhog is left out: it is mail's alias, kept beside it for whatever already read it
	for _, url := range []struct{ name, value string }{
		{"base", description.Project.URLs.Base},
		{"admin", description.Project.URLs.Admin},
		{"mail", description.Project.URLs.Mail},
		{"rabbitmq", description.Project.URLs.RabbitMQ},
		{"search", description.Project.URLs.Search},
	} {
		if url.value == "" {
			continue
		}

		status.URLs = append(status.URLs, core.URLStatus{Name: url.name, URL: url.value})
	}

	for at := range status.URLs {
		probing.Add(1)

		go func(one *core.URLStatus) {
			defer probing.Done()

			code, err := r.Probe.Answer(one.URL)
			if err != nil {
				one.Error = err.Error()

				return
			}

			one.Answers, one.Code = true, code
		}(&status.URLs[at])
	}

	for _, service := range description.Services {
		one := core.ServiceStatus{Name: service.Name, State: service.State, Running: service.State == "running"}

		// A container that is not running has nothing recent to say, and asking Compose for its
		// log is a round trip per service for nothing
		if one.Running {
			log, err := r.Logs(service.Name)
			if err != nil {
				one.LastError = "(the log could not be read: " + err.Error() + ")"
			} else {
				one.LastError = core.LastErrorLine(log)
			}
		}

		status.Services = append(status.Services, one)
	}

	probing.Wait()

	return status, nil
}
