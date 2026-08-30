package app

import (
	"fmt"
	"sort"
	"sync"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core/ports"
)

// Describer answers what a project is: versions, services, addresses and state.
//
// It is the command run most often and the one an agent runs first, so what it costs and whether
// it is right matter more than for anything else here.
type Describer struct {
	Engine  ports.ContainerEngine
	Compose ports.ComposeConfig
	Magento ports.MagentoFiles
	Tooling ports.Tooling
	State   ports.DataState
	Machine string
}

// Describe builds the whole description of a project.
func (d Describer) Describe(project core.Project, files core.ComposeFiles, withSecrets bool) (core.Description, error) {
	//
	// Six independent questions, asked at once.
	//
	// Three of them are subprocesses — the tool's version from git, Compose's own version, and
	// looking inside the php container for Xdebug — and in the shell implementation they could
	// only happen one after another. They do not depend on each other, and this is the command
	// that runs most often.
	//
	var (
		configuration core.Compose
		containers    []core.Container
		configErr     error
		enginErr      error

		version, mode, adminPath                 string
		toolVersion, composeVersion, xdebugState string
		anonymised, anonymisedAt                 string

		asking sync.WaitGroup
	)

	ask := func(work func()) {
		asking.Add(1)

		go func() {
			defer asking.Done()

			work()
		}()
	}

	ask(func() { configuration, configErr = d.Compose.Load(project.Root, project.Name, files.Load) })
	ask(func() { containers, enginErr = d.Engine.Containers() })
	ask(func() {
		version = d.Magento.Version(project.Root, project.MagentoDir)
		mode = d.Magento.Mode(project.Root, project.MagentoDir)
		adminPath = d.Magento.AdminPath(project.Root, project.MagentoDir)
	})
	ask(func() { toolVersion = d.Tooling.Version() })
	ask(func() { composeVersion = d.Tooling.ComposeVersion() })
	ask(func() { xdebugState = d.Tooling.Xdebug(project.Name) })
	ask(func() { anonymised, anonymisedAt = d.State.Anonymisation(project.Name) })

	asking.Wait()

	if configErr != nil {
		// A project whose configuration cannot be read is not a project. Saying which file and
		// why is the difference between a fixable message and "something went wrong"
		return core.Description{}, configErr
	}

	if enginErr != nil {
		return core.Description{}, enginErr
	}

	states := map[string]string{}
	running, total := 0, 0

	for _, container := range containers {
		if container.Key() != project.Name {
			continue
		}

		if container.ComposeService == "" {
			continue
		}

		states[container.ComposeService] = container.StateName
		total++

		if container.Running {
			running++
		}
	}

	description := core.Description{}
	description.Project.Name = project.Name
	description.Project.Root = project.Root
	description.Project.Domain = project.Domain
	description.Project.Status = statusOf(running, total)

	if project.Worktree != nil {
		description.Project.Worktree = project.Worktree.Name
	}

	description.Magento.Version = version
	description.Magento.Mode = mode

	for _, service := range configuration.Services {
		described := core.DescribedService{
			Name:  service.Name,
			Image: service.Image,
			State: "not created",
			Ports: []string{},
		}

		if state, ok := states[service.Name]; ok && state != "" {
			described.State = state
		}

		for _, port := range service.Ports {
			described.Ports = append(described.Ports, fmt.Sprintf("%s->%s", port.Published, port.Target))
		}

		description.Services = append(description.Services, described)
	}

	sort.Slice(description.Services, func(a, b int) bool {
		return description.Services[a].Name < description.Services[b].Name
	})

	description.Project.URLs = urlsOf(project.Domain, adminPath, configuration)

	description.Paths.MagentoDir = project.MagentoDir
	description.Paths.Workdir = d.Tooling.Workdir()
	description.Paths.Strategy = "bind mount"
	if d.Machine == "mac" {
		description.Paths.Strategy = "named volume with selective binds"
	}
	description.Paths.ComposeFiles = files.Declared

	description.Tooling.Machine = d.Machine
	description.Tooling.HmVersion = toolVersion
	description.Tooling.ComposeVersion = composeVersion
	description.Tooling.Xdebug = xdebugState

	description.Data.Anonymised = anonymised
	description.Data.AnonymisedAt = anonymisedAt

	if withSecrets {
		description.Credentials = credentialsOf(configuration)
	}

	return description, nil
}

// urlsOf builds the addresses of a project.
//
// The admin's is the one that matters: Magento generates a random front name on install unless
// told otherwise, so a project with one is a project where /admin is a 404 — and guessing it is
// how somebody reports a bug that is not there.
func urlsOf(domain, adminPath string, configuration core.Compose) core.URLs {
	urls := core.URLs{}

	if domain != "" {
		urls.Base = "https://" + domain + "/"
		urls.Admin = "https://" + domain + "/" + adminPath
	}

	mailService := "mailhog"
	for _, service := range configuration.Services {
		if service.Name == "mailpit" {
			mailService = "mailpit"

			break
		}
	}

	// `mail` is the key that does not depend on which catcher the project chose, and `mailhog` is
	// kept beside it with the same value so that anything already reading it keeps working
	urls.Mail = localURL(configuration, mailService, "8025")
	urls.Mailhog = urls.Mail
	urls.RabbitMQ = localURL(configuration, "rabbitmq", "15672")
	urls.Search = localURL(configuration, "search", "9200")

	return urls
}

func localURL(configuration core.Compose, service, target string) string {
	for _, candidate := range configuration.Services {
		if candidate.Name != service {
			continue
		}

		for _, port := range candidate.Ports {
			if port.Target == target && port.Published != "" {
				return "http://localhost:" + port.Published
			}
		}
	}

	return ""
}

func credentialsOf(configuration core.Compose) *core.Credentials {
	credentials := &core.Credentials{}

	for _, service := range configuration.Services {
		if service.Name != "db" {
			continue
		}

		credentials.Database.Name = service.Environment["MYSQL_DATABASE"]
		credentials.Database.User = service.Environment["MYSQL_USER"]
		credentials.Database.Password = service.Environment["MYSQL_PASSWORD"]
		credentials.Database.RootPassword = service.Environment["MYSQL_ROOT_PASSWORD"]
	}

	return credentials
}
