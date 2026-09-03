package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Setup creates a project's environment: the three things a project is — a name, an address and
// where its code lives — and the compose files that follow from them.
//
// The installation that comes after is a sequence of other commands, and it stays that way: what
// this owns is the environment, and what installs Magento into it is Magento's business.
type Setup struct {
	Properties ports.Properties
	FS         ports.FS
	VCS        ports.VCS
	Tooling    ports.Tooling
	Magento    ports.MagentoFiles
	Legacy     ports.Legacy
	Proxy      *Proxy

	// Root is the tool's own installation, which is where the templates and the requirements it
	// reads live.
	Root string

	// Platform is "mac" or "linux".
	Platform string

	Ask      func(question, suggestion string) (string, error)
	Choose   func(question string, options []string) (string, error)
	Announce func(string)

	Binary string
}

// Decided is what setup settled on, before anything was written.
type Decided struct {
	Name       string
	Domain     string
	MagentoDir string
	Mail       string
	Dump       string
	Install    bool
}

// Decide answers the three questions a project is, asking only what it cannot work out.
//
// A project that already has properties keeps them as the suggestions: running `setup` again on a
// configured project and pressing enter through it has to leave it exactly as it was.
func (s Setup) Decide(dir string, options core.SetupOptions, interactive bool) (Decided, error) {
	existing, _ := s.Properties.Load(dir)

	decided := Decided{
		Name:       first(options.ProjectName, existing["COMPOSE_PROJECT_NAME"]),
		Domain:     first(options.Domain, existing["DOMAIN"]),
		MagentoDir: first(options.Root, existing["MAGENTO_DIR"], "."),
		Mail:       first(options.Mail, existing["MAIL_SERVICE"], "mailhog"),
		Dump:       options.Dump,
		Install:    options.Install,
	}

	derived := core.DeriveProjectName(dir)

	if decided.Name == "" {
		answer, err := s.question("Define project name", derived, options)
		if err != nil {
			return decided, err
		}

		decided.Name = answer
	}

	decided.Name = strings.ToLower(decided.Name)

	if decided.Domain == "" {
		answer, err := s.question("Define domain", decided.Name+".local", options)
		if err != nil {
			return decided, err
		}

		decided.Domain = answer
	}

	decided.Domain = strings.ToLower(decided.Domain)

	if options.Root == "" && existing["MAGENTO_DIR"] == "" {
		answer, err := s.question("Magento root dir", decided.MagentoDir, options)
		if err != nil {
			return decided, err
		}

		decided.MagentoDir = core.MagentoRoot(answer, decided.MagentoDir)
	} else {
		decided.MagentoDir = core.MagentoRoot(decided.MagentoDir, ".")
	}

	//
	// The mail catcher is asked about only when the project has not decided yet: an existing
	// project keeps what it has, and nobody is prompted about a service they already configured.
	//
	//
	// Not asked without somebody to ask, either: mailhog is the default, so accepting everything
	// leaves a project exactly as it was built before this choice existed.
	//
	if options.Mail == "" && existing["MAIL_SERVICE"] == "" &&
		!options.UseDefault && interactive && s.Choose != nil {
		answer, err := s.Choose("Which mail catcher? (mailpit is the maintained one)",
			[]string{"mailhog", "mailpit"})
		if err != nil {
			return decided, err
		}

		decided.Mail = answer
	}

	if decided.Mail != "mailhog" && decided.Mail != "mailpit" {
		return decided, core.Refusal{
			Kind:    "unknown_mail_service",
			Code:    2,
			Message: fmt.Sprintf("'%s' is not a mail catcher this tool knows about", decided.Mail),
			Hint:    s.Binary + " setup --mail=mailpit",
		}
	}

	//
	// Where the data comes from, when nobody said. The two answers are the two ways a Magento
	// exists: somebody else's database, or a fresh installation.
	//
	//
	// This one is asked even where there is nobody to answer, and refused there rather than
	// guessed: importing somebody else's database and installing a fresh Magento are not two
	// spellings of the same thing, and picking one silently would create the wrong project.
	//
	if decided.Dump == "" && !decided.Install && !options.UseDefault && s.Choose != nil {
		answer, err := s.Choose("How do you want create database?",
			[]string{"Import sql Dump", "Magento installation"})
		if err != nil {
			return decided, err
		}

		if answer == "Import sql Dump" {
			path, err := s.dumpPath()
			if err != nil {
				return decided, err
			}

			decided.Dump = path
		}
	}

	return decided, nil
}

// dumpPath asks for a dump until it is given one that is there.
func (s Setup) dumpPath() (string, error) {
	for attempts := 0; attempts < 3; attempts++ {
		answer, err := s.Ask("Path of database dump file (sql):", "")
		if err != nil {
			return "", err
		}

		answer = core.ExpandHome(answer, os.Getenv("HOME"))

		if s.FS.Exists(answer) {
			return answer, nil
		}

		s.say(fmt.Sprintf("No such file: %s\n", answer))
	}

	return "", core.Refusal{
		Kind:    "dump_not_found",
		Code:    2,
		Message: "No database dump was given that exists",
		Hint:    s.Binary + " setup --db-dump=/path/to/dump.sql",
	}
}

// Save records what was decided, merging rather than replacing.
//
// The file is committed, so whatever it says travels to every clone of the project. The name is
// recorded only when it is a decision: writing the one the directory would have given anyway is
// what made a second clone inherit the first one's identity — same containers, same volumes,
// neither of them asked for.
func (s Setup) Save(dir string, decided Decided) error {
	existing, _ := s.Properties.Load(dir)
	had := existing["COMPOSE_PROJECT_NAME"] != ""
	derived := core.DeriveProjectName(dir)

	if err := s.Properties.Set(dir, "MAGENTO_DIR", decided.MagentoDir); err != nil {
		return err
	}

	if err := s.Properties.Set(dir, "DOMAIN", decided.Domain); err != nil {
		return err
	}

	if had || (decided.Name != derived && decided.Name != "") {
		if err := s.Properties.Set(dir, "COMPOSE_PROJECT_NAME", decided.Name); err != nil {
			return err
		}
	}

	// Mailhog is the default, so a project that runs it says nothing rather than saying the
	// default out loud
	if decided.Mail != "mailhog" {
		return s.Properties.Set(dir, "MAIL_SERVICE", decided.Mail)
	}

	return nil
}

func (s Setup) question(text, suggestion string, options core.SetupOptions) (string, error) {
	if options.UseDefault || s.Ask == nil {
		return suggestion, nil
	}

	answer, err := s.Ask(text, suggestion)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(answer) == "" {
		return suggestion, nil
	}

	return strings.TrimSpace(answer), nil
}

func (s Setup) say(message string) {
	if s.Announce != nil {
		s.Announce(message)
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

// composeFile is where a project's generated files go: beside the project, not inside the Magento
// directory, because that is where Compose is run from.
func composeFile(root, name string) string {
	return filepath.Join(root, name)
}

// Requirements is what versions this project's Magento needs, and what the person changed about
// them.
//
// The table is keyed by the version somebody's Magento resolves to rather than by the version
// itself: forty patch releases share one set of images, and a table with a row per patch would be
// forty rows to keep in step.
func (s Setup) Requirements(dir, magentoDir string, options core.SetupOptions) (map[string]string, error) {
	version := s.Magento.Version(dir, magentoDir)

	if version == "" {
		return nil, core.Refusal{
			Kind:    "no_composer_lock",
			Code:    4,
			Message: fmt.Sprintf("No composer.lock found in %s", magentoDir),
			Hint: fmt.Sprintf("Clone a project and run %s setup, or make one with %s create-project",
				s.Binary, s.Binary),
		}
	}

	s.say(fmt.Sprintf("Magento version detected: %s\n", version))

	equivalent := s.equivalent(version)
	if equivalent == "" {
		return nil, core.Refusal{
			Kind:    "unsupported_version",
			Code:    2,
			Message: "Unsupported Magento version: " + version,
			Hint:    fmt.Sprintf("Run '%s compatibility' and pass a supported version", s.Binary),
		}
	}

	table, err := core.ReadRequirements(s.FS.Read(filepath.Join(s.Root, "data", "requirements.json")))
	if err != nil {
		return nil, err
	}

	required, known := table[equivalent]
	if !known {
		return nil, core.Refusal{
			Kind:    "unsupported_version",
			Code:    2,
			Message: "Unsupported Magento version: " + version,
			Hint:    fmt.Sprintf("Run '%s compatibility' and pass a supported version", s.Binary),
		}
	}

	return required, nil
}

// equivalent is the row of the table a version resolves to.
func (s Setup) equivalent(version string) string {
	table, err := core.ReadEquivalents(
		s.FS.Read(filepath.Join(s.Root, "data", "equivalent_versions.json")))
	if err != nil {
		return ""
	}

	return table[version]
}

// Write generates the compose files a project runs on.
//
// All of them, every time, because they are generated: a machine overlay left over from an older
// version of this tool is a project running something nobody can find in the repository.
func (s Setup) Write(dir string, decided Decided, requirements map[string]string) error {
	template := s.FS.Read(filepath.Join(s.Root, "docker-compose", "docker-compose.template.yml"))
	if template == "" {
		return fmt.Errorf("the compose template is missing from %s", s.Root)
	}

	written, err := core.RenderCompose(template, requirements, decided.Mail)
	if err != nil {
		return err
	}

	version := core.ComposeVersionLine(s.Tooling.ComposeVersion())

	base := composeFile(dir, "docker-compose.yml")
	if err := write(base, strings.ReplaceAll(written, "{YML_VERSION}", version)); err != nil {
		return err
	}

	for name, template := range map[string]string{
		"docker-compose.dev.mac.yml":   "docker-compose.dev.mac.template.yml",
		"docker-compose.dev.linux.yml": "docker-compose.dev.linux.template.yml",
	} {
		contents := s.FS.Read(filepath.Join(s.Root, "docker-compose", template))
		contents = strings.ReplaceAll(contents, "{YML_VERSION}", version)
		contents = strings.ReplaceAll(contents, "{MAGENTO_DIR}", decided.MagentoDir)

		if name == "docker-compose.dev.mac.yml" {
			contents = strings.ReplaceAll(contents, "# {FILES_IN_GIT}",
				s.bindMounts(dir, decided.MagentoDir, contents))
		}

		if err := write(composeFile(dir, name), contents); err != nil {
			return err
		}
	}

	return nil
}

// bindMounts is what the repository has at its top level that PHP would not otherwise see.
//
// On macOS the code lives in a volume for speed and only the named paths are mounted from the
// host, so anything the repository tracks and nobody mounted is invisible inside the container —
// which is a file that exists in git and not in the environment, and no error explains that.
func (s Setup) bindMounts(dir, magentoDir, existing string) string {
	if s.VCS == nil {
		return ""
	}

	tracked, err := s.VCS.Tracked(filepath.Join(dir, magentoDir))
	if err != nil || len(tracked) == 0 {
		return ""
	}

	defaults := map[string]bool{}

	for name, value := range core.ReadFlags(
		s.FS.Read(filepath.Join(s.Root, "data", "default_files_magento.json"))) {
		defaults[name] = value
	}

	// The generated files themselves are never mounted: they are ours, and a project mounting its
	// own compose file into the container is a loop nobody meant to write
	defaults["docker-compose.yml"] = true
	defaults["docker-compose.dev.mac.yml"] = true
	defaults["docker-compose.dev.linux.yml"] = true
	defaults["docker-compose.proxy.yml"] = true

	return core.BindMounts(tracked, magentoDir, ":delegated", defaults, existing)
}

// write puts a generated file where it goes, making the directory above it if it is missing.
func write(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(contents), 0o644) //nolint:gosec
}

// Run creates the environment and then installs into it.
//
// The order is the shell implementation's, and every step of it exists because somebody was bitten
// by its absence: read the whole instruction first so a wrong path stops the command before
// anything is created, decide the three things a project is, write the files, and only then start
// anything.
func (s Setup) Run(dir string, options core.SetupOptions, interactive bool) error {
	if options.Dump != "" {
		options.Dump = core.ExpandHome(options.Dump, os.Getenv("HOME"))

		if !s.FS.Exists(options.Dump) {
			return core.Refusal{
				Kind:    "dump_not_found",
				Code:    2,
				Message: "No such database dump: " + options.Dump,
				Hint:    s.Binary + " setup --db-dump=/path/to/dump.sql",
			}
		}
	}

	decided, err := s.Decide(dir, options, interactive)
	if err != nil {
		return err
	}

	if err := s.Save(dir, decided); err != nil {
		return err
	}

	if s.generates(dir, options) {
		requirements, err := s.Requirements(dir, decided.MagentoDir, options)
		if err != nil {
			return err
		}

		if err := s.Write(dir, decided, requirements); err != nil {
			return err
		}
	}

	if err := s.routing(dir, decided); err != nil {
		return err
	}

	return s.install(decided)
}

// generates decides whether the compose files are written again.
//
// A compose file of ours is left alone: regenerating it recreates the containers, which is a
// morning of somebody's work for no change. One that is not ours is a project that has not been
// set up yet — an empty checkout, or a compose file from somewhere else — and that is what `setup`
// is for. Asking for it explicitly overrides both.
func (s Setup) generates(dir string, options core.SetupOptions) bool {
	base := composeFile(dir, "docker-compose.yml")

	if !s.FS.Exists(base) {
		return true
	}

	return options.Force || !strings.Contains(s.FS.Read(base), "hiberus-magento")
}

// routing writes or removes the overlay that puts this project behind the global proxy.
//
// Removing it matters as much as writing it: a project that stopped using the proxy and kept the
// file would have its ports removed by a file nobody remembers, and answer on nothing.
func (s Setup) routing(dir string, decided Decided) error {
	overlay := composeFile(dir, "docker-compose.proxy.yml")

	properties, _ := s.Properties.Load(dir)

	if !core.Truthy(properties["USE_PROXY"]) {
		if err := os.Remove(overlay); err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}

	if !core.ComposeAtLeast(s.Tooling.ComposeVersion(), core.ProxyMinCompose) {
		return core.Refusal{
			Kind: "compose_too_old",
			Code: 1,
			Message: fmt.Sprintf("The proxy needs Docker Compose %s or newer, and this is %s",
				core.ProxyMinCompose, s.Tooling.ComposeVersion()),
			Hint: "Set USE_PROXY to false in config/docker/properties.json, or update Docker",
		}
	}

	if err := write(overlay,
		core.ProxyOverlay(s.Binary, decided.Name, decided.Domain, decided.Mail)); err != nil {
		return err
	}

	if s.Proxy != nil {
		// A certificate this cannot sign is not a reason to stop: the environment is written and
		// the address works over plain HTTP until somebody signs one
		s.Proxy.Certify(decided.Domain) //nolint:errcheck
	}

	return nil
}

// install is everything that happens once the environment exists.
//
// Still the shell implementation's, command by command: three of these are not ported yet, and
// running the other three through the same door keeps the order in one place instead of half here
// and half there.
func (s Setup) install(decided Decided) error {
	//
	// The environment has to be up before anything can be installed into it. The shell
	// implementation checks whether one service answers and starts everything if it does not;
	// starting is already the operation that leaves alone what is running, so it is one step.
	//
	steps := [][]string{{"start"}, {"composer", "install"}}

	if decided.Dump != "" {
		steps = append(steps,
			[]string{"mysql", "-i", decided.Dump},
			[]string{"mysql", "-q", "DELETE FROM admin_user;"})
	}

	steps = append(steps,
		[]string{"install"},
		[]string{"magento", "setup:upgrade"},
		[]string{"magento", "deploy:mode:set", "developer"},
		[]string{"restart"},
		[]string{"ssl", decided.Domain},
		[]string{"set-host", decided.Domain, "--no-database"})

	for _, step := range steps {
		code, err := s.Legacy.Run(step)
		if err != nil {
			return err
		}

		if code != 0 {
			return core.Refusal{
				Kind:    "setup_failed",
				Code:    code,
				Message: fmt.Sprintf("`%s %s` failed, so the environment is not finished", s.Binary, strings.Join(step, " ")),
				Hint:    fmt.Sprintf("%s %s   # to run it again on its own", s.Binary, strings.Join(step, " ")),
			}
		}
	}

	return nil
}
