package app

import (
	"path/filepath"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// defaultWorkdir is where the code lives inside the PHP container when nothing says otherwise: the
// value the tool ships in data/properties.json. Written here as well because the step it is used in
// copies a project over it, and an empty one would make that the container's root.
const defaultWorkdir = "/var/www/html"

// CreateProject makes a new Magento project and the environment it runs in.
//
// It is setup with two more answers — which edition and which release — and Magento downloaded
// into the environment instead of found in a checkout, so it is setup's parts in the shell
// implementation's order: decide, write, start, and then hand over to the same install steps.
//
// The version is settled before anything else is asked or written: a release the tool has no
// images for is refused while the directory is still exactly as it was.
func (s Setup) CreateProject(dir string, options core.CreateProjectOptions, interactive bool) error {
	edition, err := s.edition(options)
	if err != nil {
		return err
	}

	version, err := s.version(options)
	if err != nil {
		return err
	}

	requirements, err := s.requirementsFor(version)
	if err != nil {
		return err
	}

	//
	// A new project always installs Magento, so the question of where its database comes from is
	// answered before Decide can ask it: there is nobody else's database to import.
	//
	decided, err := s.Decide(dir, core.SetupOptions{
		ProjectName: options.ProjectName,
		Domain:      options.Domain,
		Root:        options.Root,
		UseDefault:  options.UseDefault,
		Install:     true,
	}, interactive)
	if err != nil {
		return err
	}

	decided.InstallDefaults = options.UseDefault

	magentoDir := filepath.Join(dir, decided.MagentoDir)

	if err := s.FS.MkdirAll(magentoDir); err != nil {
		return err
	}

	if err := s.Save(dir, decided); err != nil {
		return err
	}

	if err := s.Write(dir, decided, requirements); err != nil {
		return err
	}

	if err := s.routing(dir, decided); err != nil {
		return err
	}

	//
	// Real files on the host before anything starts. On macOS, Docker Desktop creates a missing
	// bind-mount source as an empty directory inside the container, which cannot be removed from
	// in there; a file that already exists is mounted as a file. Harmless where it is not needed.
	//
	if err := s.placeholders(magentoDir, "composer.json", "composer.lock"); err != nil {
		return err
	}

	workdir := defaultWorkdir
	if properties, _ := s.Properties.Load(dir); properties["WORKDIR_PHP"] != "" {
		workdir = properties["WORKDIR_PHP"]
	}

	//
	// Composer will not create a project in a directory that is not empty, and the working
	// directory already holds the compose files, so Magento is created somewhere empty and
	// copied over.
	//
	if err := s.run([][]string{
		{"start"},
		{"exec", "sh", "-c",
			"[ -d ./var/composer_home ] && cp /var/www/.composer/auth.json ./var/composer_home/auth.json || true"},
		{"exec", "composer", "create-project", "--no-install", "--repository=https://repo.magento.com/",
			"magento/project-" + edition + "-edition=" + version, "/tmp/magento-new"},
		{"exec", "sh", "-c", "cp -R /tmp/magento-new/. " + workdir + "/"},
		{"exec", "rm", "-rf", "/tmp/magento-new"},
		{"copy-from-container", "composer.json"},
	}); err != nil {
		return err
	}

	// The lock that came with the template is somebody else's resolution, and composer install
	// has to make this project's own
	if err := s.placeholders(magentoDir, "composer.lock"); err != nil {
		return err
	}

	if err := s.run([][]string{{"restart", "phpfpm"}}); err != nil {
		return err
	}

	if err := s.install(decided); err != nil {
		return err
	}

	return s.run([][]string{{"restart"}})
}

// edition is the one given, or the one chosen.
func (s Setup) edition(options core.CreateProjectOptions) (string, error) {
	if options.Edition != "" {
		return options.Edition, nil
	}

	if options.UseDefault {
		return core.Editions[0], nil
	}

	if s.Choose == nil {
		return "", core.Refusal{
			Kind:    "input_required",
			Code:    2,
			Message: "No Magento edition was given",
			Hint:    s.Binary + " create-project -e community",
		}
	}

	return s.Choose("Magento edition:", core.Editions)
}

// version is the one given, or the one asked for with the latest supported release suggested.
func (s Setup) version(options core.CreateProjectOptions) (string, error) {
	if options.Version != "" {
		return options.Version, nil
	}

	latest := s.latest()

	return s.question("Magento version:", latest, core.SetupOptions{UseDefault: options.UseDefault})
}

// latest is the newest release the tool has images for.
func (s Setup) latest() string {
	table, err := core.ReadEquivalents(
		s.FS.Read(filepath.Join(s.Root, "data", "equivalent_versions.json")))
	if err != nil {
		return ""
	}

	versions := make([]string, 0, len(table))
	for version := range table {
		versions = append(versions, version)
	}

	return core.LatestVersion(versions)
}

// placeholders writes an empty JSON document to each file, as the shell implementation's
// `echo "{}" >` did.
func (s Setup) placeholders(dir string, names ...string) error {
	for _, name := range names {
		if err := write(filepath.Join(dir, name), "{}\n"); err != nil {
			return err
		}
	}

	return nil
}
