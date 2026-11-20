package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// AIDoctor is what AI tooling a project has, where each piece came from, and whether it is still
// that. It changes nothing.
//
// Ported from console/commands/ai-doctor.sh. Paths are built the way the script built them, by
// joining strings and not cleaning them, because the absolute path of a resource is one of the two
// keys its registration entry may be under.
type AIDoctor struct {
	// Root is the project's root (the script's HM_ROOT): its properties and its installed skills
	// and agents.
	Root string

	// Dir is where the command runs. The registration is read from here, and what it tracks is
	// looked for from here, as the script did with paths relative to $PWD.
	Dir string

	// Tool is the files the tool ships with: its skills/ and agents/ are the copies it shipped,
	// and its data/ai-platforms.json the catalog of where each platform keeps them.
	Tool fs.FS
}

// Report is the state of every resource installed for the platforms the project asks for, and of
// every skill that is tracked and no longer there.
func (d AIDoctor) Report() (core.AIDoctorReport, error) {
	report := core.AIDoctorReport{Resources: []core.AIResource{}}

	// `[ -f ]`: anything but a file there is no configuration. One that cannot be read asks for
	// no platform, as jq failing on it did
	propertiesFile := d.Root + "/" + core.AIPropertiesFile
	if info, err := os.Stat(propertiesFile); err != nil || !info.Mode().IsRegular() {
		return report, nil
	}

	report.Configured = true
	properties, _ := os.ReadFile(propertiesFile)

	registration, err := d.registration()
	if err != nil {
		return core.AIDoctorReport{}, err
	}

	catalog := core.ReadAIPlatformCatalog([]byte(asset(d.Tool, "data/ai-platforms.json")))

	for _, platform := range core.ReadAIPlatforms(properties) {
		for _, kind := range []string{"skills", "agents"} {
			directory := catalog.Directory(platform, kind)
			if directory == "" {
				continue
			}

			report.Resources = append(report.Resources, d.installed(registration, platform, kind, directory)...)
		}
	}

	for _, tracked := range registration.Tracked("skills") {
		if exists(d.fromDir(tracked)) {
			continue
		}

		report.Resources = append(report.Resources, core.AIResource{
			Platform: "-", Kind: "skills", Name: filepath.Base(tracked), State: core.AIStateMissing,
			Origin: "-", Version: "-", Installed: "-",
		})
	}

	return report, nil
}

// installed is every resource of a kind in one platform's directory, by name, hidden ones left
// out as the script's glob left them out.
func (d AIDoctor) installed(registration core.AIRegistration, platform, kind, directory string) []core.AIResource {
	path := d.Root + "/" + directory

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var resources []core.AIResource

	for _, entry := range entries {
		name := entry.Name()
		item := path + "/" + name

		if strings.HasPrefix(name, ".") || !exists(item) {
			continue
		}

		recorded, tracked := registration.Entry(kind, item, directory+"/"+name)
		if !tracked {
			// Not ours: ai-pull leaves it alone, and somebody should know which are their own
			resources = append(resources, core.AIResource{
				Platform: platform, Kind: kind, Name: name, State: core.AIStateCustom,
				Origin: "-", Version: "-", Installed: "-",
			})

			continue
		}

		current := core.DigestPath(os.DirFS(path), name)

		// For what came with the tool there is a source of truth on this machine, so whether it
		// is up to date can be answered offline
		var shipped *string
		if assetDir(d.Tool, kind+"/"+name) {
			if copies, err := fs.Sub(d.Tool, kind); err == nil {
				digest := core.DigestPath(copies, name)
				shipped = &digest
			}
		}

		resources = append(resources, core.AIResource{
			Platform: platform, Kind: kind, Name: name,
			State:     core.AIResourceState(recorded.Checksum, current, shipped),
			Origin:    recorded.Origin,
			Version:   recorded.Version,
			Installed: recorded.Installed,
		})
	}

	return resources
}

// registration is the project's registration, or an empty one when it has none.
func (d AIDoctor) registration() (core.AIRegistration, error) {
	contents, err := os.ReadFile(d.fromDir(core.AIRegistrationFile))
	if err != nil {
		if isMissing(err) {
			return core.AIRegistration{}, nil
		}

		return core.AIRegistration{}, err
	}

	return core.ReadAIRegistration(contents)
}

func (d AIDoctor) fromDir(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return d.Dir + "/" + path
}

func isMissing(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// exists is `[ -e ]`: through a link, to whatever it points at.
func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
