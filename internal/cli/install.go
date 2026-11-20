package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Installing Magento in the project's containers: console/commands/install.sh and the admin
// bootstrap it sourced (console/tasks/admin_bootstrap.sh), questions, defaults, commands and
// output included.
//
// What is decided before the install — the release and its search engine, the address, the store
// and admin settings — is core's (core/install.go). What is left here is the order, the questions
// and the two files in app/etc the install has to step around.
//

// askInstall and readVersion are the questions, replaceable so that a test can answer them in
// order without a terminal. readVersion is the bare `read` that follows a version the tool has no
// images for: no prompt, as there was none.
var (
	askInstall  = ask
	readVersion = func() (string, error) {
		answer, err := readLine(os.Stdin)

		return strings.TrimSpace(answer), err
	}
)

// installRandom is where the password and the second factor's secret come from.
var installRandom io.Reader = rand.Reader

// qrScript draws a URI as a QR code with the endroid/qr-code every Magento ships, byte for byte
// the script admin_bootstrap.sh ran.
const qrScript = `
        $uri = $argv[1] ?? "";
        if (!is_file("vendor/autoload.php")) { exit(1); }
        require "vendor/autoload.php";
        $writer = "Endroid\\QrCode\\Writer\\ConsoleWriter";
        $code   = "Endroid\\QrCode\\QrCode";
        if (!class_exists($writer) || !class_exists($code)) { exit(1); }
        $qr = method_exists($code, "create") ? $code::create($uri) : new $code($uri);
        echo (new $writer())->write($qr)->getString();
    `

func install(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	engine := newEngine(stdout, stderr, jsonOutput)

	//
	// Refused in bin/run's order, which is what the shell implementation answered with: a
	// worktree with no environment of its own, then Docker, then the project, and only then the
	// command's own options and the php container.
	//
	// From such a worktree, setup:install would install over the main checkout's database.
	//
	if resolved, err := engine.Resolve(here()); err == nil {
		if err := app.RefuseFromAnUnregisteredWorktree(resolved, "install", os.Getenv("HM_FORCE") == "1"); err != nil {
			return report(stderr, jsonOutput, "install", err)
		}
	}

	if !engine.DockerReachable() {
		return failure(stderr, jsonOutput, "install", exitDocker, "docker_unavailable",
			"Docker is not running", "Start Docker and try again")
	}

	project, code := projectOr(stderr, jsonOutput, "install")
	if code != 0 {
		return code
	}

	options, err := core.ParseInstall(args, binaryName())
	if err != nil {
		return report(stderr, jsonOutput, "install", err)
	}

	description, err := engine.Describe(project.Root, false)
	if err != nil {
		return report(stderr, jsonOutput, "install", err)
	}

	if !serviceRunning(description, phpService) {
		return failure(stderr, jsonOutput, "install", exitService, "service_not_running",
			fmt.Sprintf("Service '%s' is not running", phpService), binaryName()+" start "+phpService)
	}

	// Magento 2.4.9+ ships Valkey, and some environments name the service after it
	cacheHost := "redis"
	for _, service := range description.Services {
		if service.Name == "valkey" {
			cacheHost = "valkey"
		}
	}

	// print_info and print_warning: coloured on stdout, plain on stderr under --json, where stdout
	// is the containers'. printf'd lines stay on stdout either way, as they did.
	decorated := func(paint func(string) string, text string) {
		if jsonOutput {
			fmt.Fprint(stderr, text)

			return
		}

		fmt.Fprint(stdout, paint(text))
	}

	installer := installer{
		engine: engine, project: project, stdout: stdout, stderr: stderr,
		json: jsonOutput, decorated: decorated,
	}

	version, equivalent, err := installer.version()
	if err != nil {
		return installer.fail(err)
	}

	domain, err := installer.domain()
	if err != nil {
		return installer.fail(err)
	}

	settings, err := installer.settings(options.UseDefault)
	if err != nil {
		return installer.fail(err)
	}

	//
	// A generated password is used and never written down: ~/.hm/config.json is shared by every
	// project and stores what is answered, so a password written there becomes the next
	// project's default. One that *is* set there is respected — that is somebody's decision.
	//
	password, _ := core.InstallSetting(settings, "admin-password")
	if password == "null" {
		password = ""
	}

	if password == "" {
		if password, err = core.AdminPassword(installRandom); err != nil {
			return installer.fail(err)
		}
	}

	user, found := core.InstallSetting(settings, "admin-user")
	if !found || user == "null" {
		user = "admin"
	}

	etc := filepath.Join(project.Root, project.MagentoDir, "app", "etc")
	config, backup := filepath.Join(etc, "config.php"), filepath.Join(etc, "_config.php")

	//
	// config.php is set aside as _config.php so the install enables every module, and put back
	// over Magento's afterwards — exactly once, whether the install succeeds or fails, so no
	// backup is left for a later install to mistake for the project's:
	//
	//   neither there        nothing to set aside or put back
	//   config.php only      set aside, put back at the end
	//   _config.php only     an interrupted install's: it is the project's, put back at the end
	//   both there           refused before anything changes: the backup may be older than
	//                        config.php, and which one is the project's is somebody's decision
	//
	// The shell's mv put Magento's config.php over a backup a failed install had left.
	//
	if exists(backup) && exists(config) {
		return failure(stderr, jsonOutput, "install", exitError, "config_backup_conflict",
			"app/etc/_config.php was left by an install that did not finish, beside an app/etc/config.php that may be newer",
			"Keep the one you want (move _config.php over config.php, or delete _config.php) and run install again")
	}

	// An env.php from another install would be read instead of written
	os.RemoveAll(filepath.Join(etc, "env.php")) //nolint:errcheck

	if exists(config) {
		if err := os.Rename(config, backup); err != nil {
			return installer.fail(err)
		}
	}

	status := exitOK

	setupInstall := append([]string{"php", "./bin/magento", "setup:install"},
		core.InstallArguments(cacheHost, version, equivalent, domain)...)
	setupInstall = append(setupInstall, core.InstallSettingArguments(settings, password)...)

	for _, command := range [][]string{setupInstall, {"php", "./bin/magento", "config:set",
		"--scope=default", "--scope-code=0", "system/full_page_cache/caching_application", "2"}} {
		if status = inside(project, command, "", stdout, stderr, jsonOutput, "install"); status != exitOK {
			break
		}
	}

	if exists(backup) {
		if err := os.Rename(backup, config); err != nil {
			// Whatever the install did, the project's config.php is still set aside, and a later
			// install would refuse over it: said, with the install's own status when it failed
			code := status
			if code == exitOK {
				code = exitError
			}

			return failure(stderr, jsonOutput, "install", code, "config_restore_failed",
				"The project's app/etc/config.php could not be put back and is still app/etc/_config.php: "+err.Error(),
				"Move app/etc/_config.php over app/etc/config.php by hand")
		}
	}

	if status != exitOK {
		return status
	}

	installer.bootstrapAdmin(domain, user, password)

	return exitOK
}

// installer carries what every step of the install needs.
type installer struct {
	engine    commands
	project   core.Project
	stdout    io.Writer
	stderr    io.Writer
	json      bool
	decorated func(func(string) string, string)
}

// errInstallUnreadable marks a data file that could not be read, which is a different failure
// from Docker's.
type errInstallUnreadable struct{ error }

func (i installer) fail(err error) int {
	var unreadable errInstallUnreadable
	if errors.As(err, &unreadable) {
		return failure(i.stderr, i.json, "install", exitError, "data_unreadable", unreadable.Error(), "")
	}

	var refusal core.Refusal
	if asRefusal(err, &refusal) {
		return report(i.stderr, i.json, "install", err)
	}

	return failure(i.stderr, i.json, "install", exitError, "install_failed", err.Error(), "")
}

// question asks, or answers with the suggestion when nobody is there; with neither, it refuses
// the way the shell's custom_question did.
func (i installer) question(text, suggestion string) (string, error) {
	if os.Getenv("HM_NON_INTERACTIVE") != "" {
		if suggestion != "" {
			return suggestion, nil
		}

		return "", core.Refusal{Kind: "input_required", Code: exitUsage,
			Message: "Non-interactive mode cannot answer: " + text,
			Hint:    "Pass the value as an option, or run without --yes"}
	}

	return askInstall(text, suggestion)
}

// setting is what install.sh found in the environment: properties.json's value, which
// load_properties exported over the process's own, or the process's.
func (i installer) setting(key string) string {
	if value := i.engine.Property(i.project, key); value != "" {
		return value
	}

	return os.Getenv(key)
}

// version is the Magento release being installed and the row of the table it uses: from the
// properties, from composer.lock, or asked for with the latest supported release offered.
func (i installer) version() (string, string, error) {
	version := i.setting("MAGENTO_VERSION")
	equivalent := i.setting("EQUIVALENT_VERSION")

	if version == "" {
		lock, _ := os.ReadFile(filepath.Join(i.project.Root, i.project.MagentoDir, "composer.lock")) //nolint:gosec
		version = core.LockedMagentoVersion(string(lock))
	}

	if version == "" {
		equivalents, err := i.engine.Equivalents()
		if err != nil {
			return "", "", errInstallUnreadable{err}
		}

		latest := core.LastSupportedVersion(equivalents)

		answer, err := i.question("Magento version:", latest)
		if err != nil {
			return "", "", err
		}

		if answer == "" {
			answer = latest
		}

		version, equivalent, err = i.supported(equivalents, answer)
		if err != nil {
			return "", "", err
		}
	}

	if equivalent == "" {
		equivalent = core.EquivalentFallback(version)
	}

	return version, equivalent, nil
}

// supported looks a release up in the table and, while it is not there, says so, shows the
// table and reads another — or refuses, with nobody to read one from.
func (i installer) supported(equivalents map[string]string, version string) (string, string, error) {
	for {
		if equivalent, ok := equivalents[version]; ok {
			return version, equivalent, nil
		}

		if os.Getenv("HM_NON_INTERACTIVE") != "" {
			return "", "", core.Refusal{Kind: "input_required", Code: exitUsage,
				Message: "Unsupported Magento version: " + version,
				Hint:    fmt.Sprintf("Run '%s compatibility' and pass a supported version", binaryName())}
		}

		i.decorated(warning, fmt.Sprintf("\nWe don't have support for the version %s ", version))
		i.decorated(good, "\nPlease, write any version between all versions supported or press Ctrl - C to exit")

		table, err := i.engine.Compatibility()
		if err != nil {
			return "", "", errInstallUnreadable{err}
		}

		fmt.Fprint(i.stdout, paint("\033[0;36m", table))

		answer, err := readVersion()
		if err != nil && answer == "" {
			return "", "", core.Refusal{Kind: "input_required", Code: exitUsage,
				Message: "Unsupported Magento version: " + version,
				Hint:    fmt.Sprintf("Run '%s compatibility' and pass a supported version", binaryName())}
		}

		version = answer
	}
}

// domain is the project's, or asked for with <project>.local offered, lowercased either way.
func (i installer) domain() (string, error) {
	domain := i.project.Domain
	if domain == "" {
		domain = os.Getenv("DOMAIN")
	}

	if domain == "" {
		suggestion := i.project.Name + ".local"

		answer, err := i.question("Define domain", suggestion)
		if err != nil {
			return "", err
		}

		domain = answer
		if domain == "" {
			domain = suggestion
		}
	}

	return strings.ToLower(domain), nil
}

// settings is ~/.hm/config.json, after asking about each of its questions unless the defaults were
// asked for. Every answer is written back as it is given, as get_argument_command did.
func (i installer) settings(useDefault bool) ([]core.Setting, error) {
	settings, err := i.engine.InstallSettings()
	if err != nil {
		return nil, errInstallUnreadable{err}
	}

	if useDefault {
		return settings, nil
	}

	for _, key := range core.InstallQuestions {
		current, _ := core.InstallSetting(settings, key)

		// A value jq -r printed as null, or an empty one, is no default at all
		suggestion := current
		if suggestion == "null" {
			suggestion = ""
		}

		answer, err := i.question("Define "+key, suggestion)
		if err != nil {
			return nil, err
		}

		if answer == "" {
			answer = current
		}

		settings = core.SetInstallSetting(settings, key, answer)

		if err := i.engine.SaveInstallSettings(settings); err != nil {
			return nil, err
		}
	}

	return settings, nil
}

// bootstrapAdmin registers the second factor when the project has the module on, and prints what
// is needed to log in the first time, in one block at the end where nothing pushes it off screen.
func (i installer) bootstrapAdmin(domain, user, password string) {
	uri, secret := "", ""

	if i.twoFactorEnabled() {
		var registered bool

		uri, secret, registered = i.registerSecondFactor(domain, user)
		if !registered {
			i.decorated(warning, fmt.Sprintf("Could not register the second factor for '%s'\n", user))
		}
	} else {
		i.decorated(good, "Two factor authentication is disabled in this project, so none was set up.\n")
	}

	out := i.stdout

	fmt.Fprint(out, "\n")
	i.decorated(bold, "Your environment is ready\n")
	fmt.Fprint(out, "\n")

	fmt.Fprintf(out, "  %-12s ", "storefront")
	fmt.Fprint(out, link("https://"+domain+"/\n"))
	fmt.Fprintf(out, "  %-12s ", "admin")
	fmt.Fprint(out, link("https://"+domain+"/admin\n"))
	fmt.Fprint(out, "\n")
	fmt.Fprintf(out, "  %-12s %s\n", "user", user)
	fmt.Fprintf(out, "  %-12s %s\n", "password", password)
	fmt.Fprint(out, "\n")
	i.decorated(warning, "This password is not stored anywhere. Save it now.\n")

	if uri != "" {
		fmt.Fprint(out, "\n")
		i.decorated(good, "Scan this with your authenticator app:\n\n")

		drawn, status, err := i.engine.Capture(i.project.Root, phpService, []string{"php", "-r", qrScript, "--", uri})
		fmt.Fprint(out, drawn)

		if err != nil || status != 0 {
			i.decorated(warning, "  The code could not be drawn. Enter it by hand instead.\n")
		}

		// The key on its own as well: scanning is not always possible, and every app offers to
		// type it in
		fmt.Fprint(out, "\n")
		fmt.Fprintf(out, "  %-10s %s\n", "account", domain+":"+user)
		fmt.Fprintf(out, "  %-10s %s\n", "key", secret)
		fmt.Fprintf(out, "  %-10s %s\n", "type", "time based (TOTP)")
		fmt.Fprint(out, "\n")
		fmt.Fprintf(out, "  %-10s %s\n", "uri", uri)
	}

	fmt.Fprint(out, "\n")
}

// twoFactorEnabled is `magento module:status Magento_TwoFactorAuth | grep -qi "module is enabled"`.
func (i installer) twoFactorEnabled() bool {
	output, _, err := i.engine.Capture(i.project.Root, phpService,
		[]string{"php", "./bin/magento", "module:status", "Magento_TwoFactorAuth"})

	return err == nil && strings.Contains(strings.ToLower(output), "module is enabled")
}

// registerSecondFactor registers a new TOTP secret for the user with the module's own command,
// and answers the registration URI — empty when it could not be registered — and the secret.
func (i installer) registerSecondFactor(domain, user string) (string, string, bool) {
	issuer := domain
	if issuer == "" {
		issuer = "magento"
	}

	secret, err := core.OTPSecret(installRandom)
	if err != nil {
		return "", "", false
	}

	_, status, err := i.engine.Capture(i.project.Root, phpService,
		[]string{"php", "./bin/magento", "security:tfa:google:set-secret", user, secret})
	if err != nil || status != 0 {
		return "", secret, false
	}

	return core.OTPURI(issuer, user, secret), secret, true
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
