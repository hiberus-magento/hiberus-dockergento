package core

// Container is one Docker container, seen through the labels this tool stamps on the ones it
// creates.
//
// The inventory is built from labels and not from a registry file, so it works from any directory
// and cannot drift from what is actually running — a registry can say an environment exists after
// somebody removed its containers by hand.
type Container struct {
	ID      string
	Running bool

	// Name is what Docker calls it, which is how the environment holding a port is named to
	// somebody who has to go and stop it.
	Name string

	// ComposeProject and ComposeService are what Compose stamps on everything it creates.
	ComposeProject string
	ComposeService string

	// Project, Root, Worktree, Magento are ours. A container with them was created by this tool;
	// one without may still belong to a project of ours, created before the labels existed.
	Project  string
	Root     string
	Worktree string
	Magento  string

	// WorkingDir is Compose's own record of where the project was, used when ours is missing.
	WorkingDir string

	// StateName is Docker's own word for the state — running, exited, created — which is what a
	// description reports, as opposed to the boolean the inventory needs.
	StateName string

	// Published is the ports this container holds on the host, which is how a port conflict is
	// attributed to the environment causing it instead of reported as "something is listening".
	Published []string

	// Networks is what it is attached to, which is where anything that has to reach it has to be
	// attached too.
	Networks []string

	// Mounts is what is mounted into it, and where. Read from the container rather than from the
	// configuration on purpose: what matters is what the running container actually has.
	Mounts []Mount
}

// Bind is a directory of this machine made visible inside a container.
type Bind struct {
	Source string
	Target string
}

// Mount is one thing mounted into a container.
type Mount struct {
	// Type is bind, volume or tmpfs. The distinction is the whole point of the check that uses
	// it: dependencies on a bind mount are dependencies PHP resolves through the host.
	Type string

	// Destination is where it appears inside the container.
	Destination string
}

// DaemonInfo is what the daemon says about itself: the memory and CPUs the containers actually
// have, and what is providing them.
type DaemonInfo struct {
	MemoryBytes int64
	CPUs        int

	// Runtime is the daemon's own name — colima, docker-desktop — which is what decides whether
	// there is an instruction to give for making it bigger.
	Runtime string
}

// Listener is a process holding a port on this machine.
type Listener struct {
	Port string

	// Process is empty when the tool that listed the port does not name it, and the message says
	// "processes on the host" rather than inventing one.
	Process string
}

// Key is the name the container's environment is known by.
func (c Container) Key() string {
	if c.Project != "" {
		return c.Project
	}

	return c.ComposeProject
}

// Environment is a project's containers seen as one thing.
type Environment struct {
	Name        string `json:"name"`
	Root        string `json:"root"`
	Worktree    string `json:"worktree"`
	Magento     string `json:"magento"`
	HasMetadata bool   `json:"has_metadata"`

	Containers struct {
		Running int `json:"running"`
		Total   int `json:"total"`
	} `json:"containers"`

	Branch string `json:"branch"`

	// Orphan is an environment whose directory is gone: containers and volumes with nothing left
	// to belong to.
	Orphan bool `json:"orphan"`

	// Status is running, stopped, or partial — which is its own answer and not a rounding of the
	// other two: a project with half its services up is the state people actually get stuck in.
	Status string `json:"status"`
}

// Compose is a project's resolved Compose configuration: what the files say after being merged,
// interpolated and validated.
type Compose struct {
	Name     string
	Services []Service

	// Volumes maps the name in the file to the name Docker gives it.
	Volumes map[string]string
}

// ComposeFiles is what a project is built from: everything that has to be loaded to resolve it,
// and the ones the project itself declares.
//
// They are not the same list, and reporting the wrong one would be a small lie with consequences:
// a project routed through the global proxy carries a third file that removes its published ports
// and adds the routing, and a branch environment carries an overlay that lives outside the
// checkout. Both have to be loaded; neither is something the project declared.
type ComposeFiles struct {
	Load     []string
	Declared []string
}

// Service is one service of that configuration.
type Service struct {
	Name        string
	Image       string
	Ports       []Port
	Environment map[string]string
}

// Port is a published port: what it is on the host and what it is in the container.
type Port struct {
	Published string
	Target    string
}

// Description is everything that defines a project: what `hm describe` answers.
type Description struct {
	Project struct {
		Name     string `json:"name"`
		Root     string `json:"root"`
		Worktree string `json:"worktree"`
		Domain   string `json:"domain"`
		Status   string `json:"status"`
		URLs     URLs   `json:"urls"`
	} `json:"project"`

	Magento struct {
		Version string `json:"version"`
		Mode    string `json:"mode"`
	} `json:"magento"`

	Data struct {
		Anonymised   string `json:"anonymised"`
		AnonymisedAt string `json:"anonymised_at"`
	} `json:"data"`

	Services []DescribedService `json:"services"`

	Paths struct {
		MagentoDir   string   `json:"magento_dir"`
		Workdir      string   `json:"workdir"`
		Strategy     string   `json:"strategy"`
		ComposeFiles []string `json:"compose_files"`
	} `json:"paths"`

	Tooling struct {
		Machine        string `json:"machine"`
		HmVersion      string `json:"hm_version"`
		ComposeVersion string `json:"compose_version"`
		Xdebug         string `json:"xdebug"`
	} `json:"tooling"`

	// Credentials are only there when they were asked for. A description that carried the
	// database password by default would end up in logs, in issues and in an agent's context.
	Credentials *Credentials `json:"credentials,omitempty"`
}

// URLs are the addresses of a project.
type URLs struct {
	Base     string `json:"base"`
	Admin    string `json:"admin"`
	Mail     string `json:"mail"`
	Mailhog  string `json:"mailhog"`
	RabbitMQ string `json:"rabbitmq"`
	Search   string `json:"search"`
}

// DescribedService is a service with the state its container is actually in.
type DescribedService struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	State string   `json:"state"`
	Ports []string `json:"ports"`
}

// Credentials are the database's, and nothing else: they are asked for, never volunteered.
type Credentials struct {
	Database struct {
		Name         string `json:"name"`
		User         string `json:"user"`
		Password     string `json:"password"`
		RootPassword string `json:"root_password"`
	} `json:"database"`
}
