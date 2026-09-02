package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Templates: the data directory of a database, frozen and cloned as files.
//
// Nothing here talks to a database server. It replaces the files underneath one, which is what
// makes it seconds instead of the tens of minutes an import costs — and also why the environment
// has to be down for it.
type Templates struct {
	Volumes      ports.VolumeStore
	OneOff       ports.OneOff
	Engine       ports.ContainerEngine
	Orchestrator ports.Orchestrator
	State        ports.DataState

	Progress func(label string) func(ok bool, note string)
	Announce func(string)
	Ask      func(question, suggestion string) (string, error)

	Binary string
}

// List is every template on this machine, whichever project made it.
func (t Templates) List() ([]core.Template, error) {
	volumes, err := t.Volumes.Labelled(core.TemplateLabel)
	if err != nil {
		return nil, err
	}

	templates := make([]core.Template, 0, len(volumes))

	for _, labels := range volumes {
		bytes, _ := strconv.ParseInt(labels["hm.bytes"], 10, 64)

		templates = append(templates, core.Template{
			Project: labels["hm.project"],
			Name:    labels[core.TemplateLabel],
			Address: core.TemplateAddress(labels["hm.project"], labels[core.TemplateLabel]),
			Size:    labels["hm.size"],
			Bytes:   bytes,
			Image:   labels["hm.db_image"],
			Created: labels["hm.created"],
			Volume:  labels["name"],
		})
	}

	sort.Slice(templates, func(a, b int) bool {
		return templates[a].Volume < templates[b].Volume
	})

	return templates, nil
}

// FreezeOptions is what a freeze was asked for.
type FreezeOptions struct {
	Name  string
	Force bool
}

// Freeze copies this project's data directory into a template.
func (t Templates) Freeze(project core.Project, files core.ComposeFiles,
	configuration core.Compose, options FreezeOptions) (core.Template, error) {
	name := options.Name
	if name == "" {
		name = "base"
	}

	if !core.ValidTemplateName(name) {
		return core.Template{}, invalidName(name)
	}

	volume, image := dataDirectory(configuration)
	if volume == "" || image == "" {
		return core.Template{}, t.noDatabaseService()
	}

	if !t.Volumes.Exists(volume) {
		return core.Template{}, core.Refusal{
			Kind: "no_data", Code: 6,
			Message: "This project has no data directory yet, so there is nothing to freeze",
			Hint:    t.Binary + " start",
		}
	}

	template := core.TemplateVolume(project.Name, name)
	address := core.TemplateAddress(project.Name, name)

	if t.Volumes.Exists(template) && !options.Force {
		return core.Template{}, core.Refusal{
			Kind: "template_exists", Code: 2,
			Message: fmt.Sprintf("There is already a template called '%s'", address),
			Hint:    fmt.Sprintf("%s db freeze --name=%s --force", t.Binary, name),
		}
	}

	t.announce("Measuring the data directory...\n")

	bytes := t.measure(volume, image)
	size := core.HumanSize(bytes)

	//
	// Only the database goes down, and only for the copy. Whatever happens next it comes back up:
	// leaving somebody's environment half stopped because a disk filled up would be worse than
	// the failure itself.
	//
	running := t.databaseIsRunning(project)
	label := fmt.Sprintf("Copying the data directory (%s)...", size)

	if running {
		label = fmt.Sprintf("The database is stopped while it copies (%s)...", size)

		if err := t.Orchestrator.Stop(project, files, []string{"db"}); err != nil {
			return core.Template{}, err
		}
	}

	done := t.begin(label)

	if options.Force && t.Volumes.Exists(template) {
		t.Volumes.Remove(template) //nolint:errcheck
	}

	if err := t.Volumes.Create(template, map[string]string{
		core.TemplateLabel: name,
		"hm.project":       project.Name,
		"hm.root":          project.Root,
		"hm.db_image":      image,
		"hm.created":       time.Now().Format("2006-01-02 15:04"),
		"hm.bytes":         strconv.FormatInt(bytes, 10),
		"hm.size":          size,
	}); err != nil {
		done(false, "")

		return core.Template{}, err
	}

	err := t.copy(volume, template, image)

	done(err == nil, "")

	if running {
		t.Orchestrator.Up(project, files, []string{"db"}) //nolint:errcheck
	}

	if err != nil {
		t.Volumes.Remove(template) //nolint:errcheck

		return core.Template{}, core.Refusal{
			Kind: "freeze_failed", Code: 6,
			Message: "The data directory could not be copied",
			Hint:    t.Binary + " doctor",
		}
	}

	return core.Template{
		Project: project.Name, Name: name, Address: address,
		Size: size, Bytes: bytes, Image: image, Volume: template,
	}, nil
}

// Clone builds this project's data directory from a template.
func (t Templates) Clone(project core.Project, configuration core.Compose,
	address string, force bool) (core.Template, error) {
	if strings.HasPrefix(address, "-") {
		return core.Template{}, core.Refusal{
			Kind: "invalid_argument", Code: 2,
			Message: "Unknown option: " + address,
			Hint:    t.Binary + " db clone base",
		}
	}

	if address == "" {
		address = "base"
	}

	from, name := core.ParseTemplate(address, project.Name)

	if !core.ValidTemplateName(name) {
		return core.Template{}, invalidName(name)
	}

	template := core.TemplateVolume(from, name)

	if !t.Volumes.Exists(template) {
		return core.Template{}, t.unknownTemplate(from, name)
	}

	//
	// Nothing may be running. This replaces the files a server has open, and a server that finds
	// its data directory changed underneath it does not notice until much later.
	//
	if t.anythingIsRunning(project) {
		return core.Template{}, core.Refusal{
			Kind: "environment_running", Code: 6,
			Message: fmt.Sprintf("The database's files cannot be replaced while '%s' is running", project.Name),
			Hint:    t.Binary + " stop",
		}
	}

	volume, image := dataDirectory(configuration)
	if volume == "" || image == "" {
		return core.Template{}, t.noDatabaseService()
	}

	//
	// A data directory is not portable across server versions: 10.6 files under 10.2 produce a
	// server that starts, complains, and loses data in ways that are found much later.
	//
	if made := t.Volumes.Labels(template)["hm.db_image"]; made != "" && made != image && !force {
		return core.Template{}, core.Refusal{
			Kind: "image_mismatch", Code: 6,
			Message: fmt.Sprintf("That template was made with %s and this project runs %s", made, image),
			Hint:    "Freeze a template from a project on the same image, or insist with --force",
		}
	}

	if !force && t.hasData(volume, image) {
		confirmed, err := t.confirmReplacing(project, address)
		if err != nil {
			return core.Template{}, err
		}

		if !confirmed {
			return core.Template{}, nil
		}
	}

	done := t.begin("Cloning " + address + "...")

	// Whatever the template holds, nobody anonymised it here
	t.State.Clear(project.Name) //nolint:errcheck

	err := t.copy(template, volume, image)

	done(err == nil, "")

	if err != nil {
		return core.Template{}, core.Refusal{
			Kind: "clone_failed", Code: 6,
			Message: fmt.Sprintf("The template could not be copied into '%s'", project.Name),
			Hint:    t.Binary + " doctor",
		}
	}

	return core.Template{Address: address, Project: project.Name, Volume: volume}, nil
}

// Drop deletes a template.
func (t Templates) Drop(project core.Project, address string, force, interactive bool) (core.Template, error) {
	if address == "" {
		return core.Template{}, core.Refusal{
			Kind: "missing_name", Code: 2,
			Message: "Which template should be dropped?",
			Hint:    t.Binary + " db templates",
		}
	}

	from, name := core.ParseTemplate(address, project.Name)

	if !core.ValidTemplateName(name) {
		return core.Template{}, invalidName(name)
	}

	template := core.TemplateVolume(from, name)

	if !t.Volumes.Exists(template) {
		return core.Template{}, core.Refusal{
			Kind: "unknown_template", Code: 2,
			Message: fmt.Sprintf("There is no template called '%s'", core.TemplateAddress(from, name)),
			Hint:    t.Binary + " db templates",
		}
	}

	// A stopped container still holds a volume, which is why Docker refuses to remove it. Saying
	// so beforehand is better than relaying that error
	users, err := t.Volumes.Users(template)
	if err != nil {
		return core.Template{}, err
	}

	if len(users) > 0 {
		return core.Template{}, core.Refusal{
			Kind: "template_in_use", Code: 6,
			Message: fmt.Sprintf("That template is attached to a container: %s", strings.Join(users, " ")),
			Hint:    "Remove that container first, or clone the template instead of mounting it",
		}
	}

	full := core.TemplateAddress(from, name)
	size := t.Volumes.Labels(template)["hm.size"]

	if interactive && !force {
		t.announce(fmt.Sprintf("This deletes the template %s (%s).\n\n", full, size))

		answer, err := t.Ask("Delete it? [y/N]", "")
		if err != nil {
			return core.Template{}, err
		}

		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
		default:
			return core.Template{}, nil
		}
	}

	if err := t.Volumes.Remove(template); err != nil {
		return core.Template{}, err
	}

	return core.Template{Address: full, Size: size}, nil
}

// The volume compose will mount as the database's data directory, and the image it will run.
//
// Both come from the resolved configuration rather than from a name built here: the project name
// can be overridden, the volume can be renamed in an overlay, and a guess that is right for most
// projects is the kind of thing that destroys the data of the rest.
// CopyVolume copies one volume over another with the given image, which is how a branch
// environment on macOS gets the code: there the code lives in a named volume and nothing on this
// filesystem can be mounted into it.
func (t Templates) CopyVolume(from, to, image string) error { return t.copy(from, to, image) }

// DataDirectory is the volume compose will mount as the database's data directory, and the image
// it will run.
func DataDirectory(configuration core.Compose) (string, string) { return dataDirectory(configuration) }

func dataDirectory(configuration core.Compose) (string, string) {
	volume := configuration.Volumes["dbdata"]

	image := ""

	for _, service := range configuration.Services {
		if service.Name == databaseService {
			image = service.Image
		}
	}

	return volume, image
}

// measure is done from inside a container because on macOS the volume lives in a virtual machine
// and its mountpoint does not exist out here.
func (t Templates) measure(volume, image string) int64 {
	var answer strings.Builder

	if _, err := t.OneOff.Volumes(image, []string{"-c", "du -s -B1 /from | cut -f1"},
		map[string]string{volume: "/from:ro"}, &answer); err != nil {
		return 0
	}

	bytes, _ := strconv.ParseInt(strings.TrimSpace(answer.String()), 10, 64)

	return bytes
}

// copy one data directory over another.
//
// The destination is emptied first: a data directory restored on top of another is neither of the
// two, and InnoDB will start on the mixture and only say so later.
//
// The project's own database image does it. It is already on the machine, so nothing is pulled,
// and its GNU `cp -a` reproduces a data directory — ownership, sockets, sparse files — where
// busybox's would need arguing with.
func (t Templates) copy(from, to, image string) error {
	status, err := t.OneOff.Volumes(image, []string{"-c", `
        if [ -z "$(ls -A /from)" ]; then exit 3; fi
        find /to -mindepth 1 -delete 2>/dev/null || true
        cp -a /from/. /to/
    `}, map[string]string{from: "/from:ro", to: "/to"}, nil)
	if err != nil {
		return err
	}

	if status != 0 {
		return fmt.Errorf("the copy ended with status %d", status)
	}

	return nil
}

func (t Templates) hasData(volume, image string) bool {
	if !t.Volumes.Exists(volume) {
		return false
	}

	// A megabyte is an empty data directory with its own bookkeeping in it
	return t.measure(volume, image) > 1048576
}

// confirmReplacing asks for the project's name rather than a letter: a blind `y` is a reflex, and
// typing the name means the sentence was read.
func (t Templates) confirmReplacing(project core.Project, address string) (bool, error) {
	t.announce(fmt.Sprintf("This replaces the database of '%s' with %s.\n", project.Name, address))
	t.announce("Everything in it will be lost.\n\n")

	answer, err := t.Ask("Type the project name to confirm", "")
	if err != nil {
		return false, core.Refusal{
			Kind: "would_replace_data", Code: 6,
			Message: fmt.Sprintf("'%s' already has a database and this would replace it", project.Name),
			Hint:    fmt.Sprintf("%s db clone %s --force", t.Binary, address),
		}
	}

	return strings.TrimSpace(answer) == project.Name, nil
}

func (t Templates) databaseIsRunning(project core.Project) bool {
	return t.running(project, databaseService)
}

func (t Templates) anythingIsRunning(project core.Project) bool {
	return t.running(project, "")
}

func (t Templates) running(project core.Project, service string) bool {
	containers, err := t.Engine.Containers()
	if err != nil {
		return false
	}

	for _, container := range containers {
		if container.Key() != project.Name || !container.Running {
			continue
		}

		if service == "" || container.ComposeService == service {
			return true
		}
	}

	return false
}

func (t Templates) unknownTemplate(project, name string) error {
	known := []string{}

	if templates, err := t.List(); err == nil {
		for _, template := range templates {
			known = append(known, "  "+template.Address)
		}
	}

	if len(known) == 0 {
		known = []string{"  (none yet: " + t.Binary + " db freeze)"}
	}

	return core.Refusal{
		Kind: "unknown_template", Code: 2,
		Message: fmt.Sprintf("There is no template called '%s'", core.TemplateAddress(project, name)),
		Hint:    "Templates on this machine:\n" + strings.Join(known, "\n"),
	}
}

func (t Templates) noDatabaseService() error {
	return core.Refusal{
		Kind: "no_database_service", Code: 4,
		Message: "This project's configuration defines no database service",
		Hint:    t.Binary + " setup -f",
	}
}

func invalidName(name string) error {
	return core.Refusal{
		Kind: "invalid_name", Code: 2,
		Message: fmt.Sprintf("'%s' cannot be used as a snapshot name", name),
		Hint:    "Letters, digits, dots, dashes and underscores, not starting with a dot or a dash",
	}
}

func (t Templates) announce(message string) {
	if t.Announce != nil {
		t.Announce(message)
	}
}

func (t Templates) begin(label string) func(bool, string) {
	if t.Progress == nil {
		return func(bool, string) {}
	}

	return t.Progress(label)
}
