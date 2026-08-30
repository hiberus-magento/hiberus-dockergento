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
