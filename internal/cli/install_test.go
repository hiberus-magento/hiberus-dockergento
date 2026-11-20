package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// install installs Magento in the project's containers — console/commands/install.sh with
// admin_bootstrap.sh, questions, defaults, commands and output included. Never a daemon: the
// engine is a fake, and the commands it was asked to run are what is checked.
//

// The two Magento commands install.sh ran around the install, and the one that asks whether the
// second factor is on.
var (
	fullPageCache = []string{"php", "./bin/magento", "config:set", "--scope=default", "--scope-code=0",
		"system/full_page_cache/caching_application", "2"}
	twoFactorStatus = []string{"php", "./bin/magento", "module:status", "Magento_TwoFactorAuth"}
)

// The settings ~/.hm/config.json is created with, as options of setup:install.
func defaultSettingArguments(password string) []string {
	return []string{"--language=es_ES", "--currency=EUR", "--timezone=Europe/Madrid",
		"--admin-firstname=hiberus", "--admin-lastname=hiberus", "--admin-email=noreply@hiberus.com",
		"--admin-user=hiberus", "--admin-password=" + password}
}

// installLab is a project at 2.4.7-p1 whose phpfpm is running, with an env.php and a config.php
// for the install to deal with.
func installLab(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("MAGENTO_VERSION", "")
	t.Setenv("EQUIVALENT_VERSION", "")
	t.Setenv("DOMAIN", "")

	fake.project = core.Project{Name: "shop", Root: here(), Domain: "shop.test", MagentoDir: "."}
	fake.description.Services = []core.DescribedService{
		{Name: "db", State: "running"},
		{Name: "phpfpm", State: "running"},
		{Name: "redis", State: "running"},
	}
	fake.equivalents = map[string]string{"2.3.7": "2.3.7", "2.4.5": "2.4.5", "2.4.7": "2.4.7", "2.4.7-p1": "2.4.7"}

	writeFile(t, "composer.lock", `{"packages": [{"name": "magento/product-community-edition", "version": "2.4.7-p1"}]}`)
	writeFile(t, "app/etc/env.php", "<?php return [];")
	writeFile(t, "app/etc/config.php", "<?php return ['modules' => []];")

	return fake
}

func writeFile(t *testing.T, name, contents string) {
	t.Helper()

	path := filepath.Join(here(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runInstall runs the command and answers what it wrote.
func runInstall(t *testing.T, jsonOutput bool, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := install(args, &stdout, &stderr, jsonOutput)

	return code, stdout.String(), stderr.String()
}

// commandsRun is every Exec and Capture, in order, as "Method service argv".
func commandsRun(fake *fakeEngine) [][]string {
	out := [][]string{}

	for _, one := range fake.calls {
		if one.Method == "Exec" || one.Method == "Capture" {
			out = append(out, append([]string{one.Method, one.Service}, one.Command...))
		}
	}

	return out
}

// installedWith is the setup:install command that was run, and the password it was given.
func installedWith(t *testing.T, fake *fakeEngine) ([]string, string) {
	t.Helper()

	for _, one := range fake.calls {
		if one.Method == "Exec" && len(one.Command) > 2 && one.Command[2] == "setup:install" {
			last := one.Command[len(one.Command)-1]

			return one.Command, strings.TrimPrefix(last, "--admin-password=")
		}
	}

	t.Fatalf("setup:install was never run: %+v", fake.calls)

	return nil, ""
}

func summary(user, password string) string {
	return "\nYour environment is ready\n\n" +
		"  storefront   https://shop.test/\n" +
		"  admin        https://shop.test/admin\n\n" +
		"  user         " + user + "\n" +
		"  password     " + password + "\n\n" +
		"This password is not stored anywhere. Save it now.\n"
}

// TestInstallRunsSetupInstallWithEverythingItDecided: with nobody to ask, the version comes from
// composer.lock, the domain from the project, the settings from a ~/.hm/config.json created with
// the defaults, and a generated password. The exact commands, in order.
func TestInstallRunsSetupInstallWithEverythingItDecided(t *testing.T) {
	fake := installLab(t)

	code, stdout, stderr := runInstall(t, false)
	if code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	_, password := installedWith(t, fake)
	if !regexp.MustCompile(`^[A-Za-z0-9]{18}[A-Za-z][0-9]$`).MatchString(password) {
		t.Fatalf("generated password %q has the wrong shape", password)
	}

	setupInstall := append([]string{"Exec", "phpfpm", "php", "./bin/magento", "setup:install"},
		core.InstallArguments("redis", "2.4.7-p1", "2.4.7", "shop.test")...)
	setupInstall = append(setupInstall, defaultSettingArguments(password)...)

	want := [][]string{
		setupInstall,
		append([]string{"Exec", "phpfpm"}, fullPageCache...),
		append([]string{"Capture", "phpfpm"}, twoFactorStatus...),
	}

	if diff := cmp.Diff(want, commandsRun(fake)); diff != "" {
		t.Fatalf("commands (-want +got):\n%s", diff)
	}

	for _, one := range fake.calls {
		if one.Method == "Exec" && !cmp.Equal(one.Options, core.ExecOptions{Interactive: true}) {
			t.Errorf("exec options = %+v, want what `hm magento` asks for without a terminal", one.Options)
		}
	}

	wantOut := "Two factor authentication is disabled in this project, so none was set up.\n" +
		summary("hiberus", password) + "\n"
	if stdout != wantOut {
		t.Errorf("stdout:\n%q\nwant:\n%q", stdout, wantOut)
	}

	// The defaults are what the file now holds, and the password is not among them
	if diff := cmp.Diff(core.DefaultInstallSettings(), fake.settings); diff != "" {
		t.Errorf("settings (-want +got):\n%s", diff)
	}
}

// TestInstallRemovesEnvAndPutsConfigBack: env.php is gone, and config.php is set aside during the
// install and put back over the one Magento wrote.
func TestInstallRemovesEnvAndPutsConfigBack(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if len(command) > 2 && command[2] == "setup:install" {
			// What Magento leaves behind
			writeFile(t, "app/etc/config.php", "<?php // Magento's")
			writeFile(t, "app/etc/env.php", "<?php // Magento's")

			if _, err := os.Stat(filepath.Join(here(), "app/etc/_config.php")); err != nil {
				t.Errorf("config.php was not set aside before the install: %v", err)
			}
		}

		return outcome{}, false
	}

	if code, _, stderr := runInstall(t, false); code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	config, _ := os.ReadFile(filepath.Join(here(), "app/etc/config.php"))
	if string(config) != "<?php return ['modules' => []];" {
		t.Errorf("config.php = %q, want the project's own back", config)
	}

	if _, err := os.Stat(filepath.Join(here(), "app/etc/_config.php")); !os.IsNotExist(err) {
		t.Errorf("the backup is still there: %v", err)
	}
}

// TestInstallWithABackupLeftAloneUsesIt: an install that was interrupted can leave the project's
// config.php set aside and none in its place. That backup is the project's config and is what
// comes back, once.
func TestInstallWithABackupLeftAloneUsesIt(t *testing.T) {
	installLab(t)
	writeFile(t, "app/etc/_config.php", "<?php // the project's")

	if err := os.Remove(filepath.Join(here(), "app/etc/config.php")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if code, _, stderr := runInstall(t, false); code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	config, _ := os.ReadFile(filepath.Join(here(), "app/etc/config.php"))
	if string(config) != "<?php // the project's" {
		t.Errorf("config.php = %q, want the project's own back", config)
	}

	if _, err := os.Stat(filepath.Join(here(), "app/etc/_config.php")); !os.IsNotExist(err) {
		t.Errorf("the backup is still there: %v", err)
	}
}

// TestInstallRefusesABackupBesideAConfig: a backup left by an install that did not finish, beside
// a config.php that may be newer than it, cannot be told apart from a stale one. Neither is
// overwritten: install refuses before it changes anything and says how to choose.
func TestInstallRefusesABackupBesideAConfig(t *testing.T) {
	fake := installLab(t)
	writeFile(t, "app/etc/_config.php", "<?php // an old backup")
	writeFile(t, "app/etc/config.php", "<?php // somebody's newer one")
	writeFile(t, "app/etc/env.php", "<?php // env")

	code, _, stderr := runInstall(t, false)
	if code != exitError {
		t.Fatalf("install = %d, want 1", code)
	}

	if !strings.Contains(stderr, "app/etc/_config.php was left by an install that did not finish") {
		t.Errorf("stderr = %q", stderr)
	}

	for name, want := range map[string]string{
		"app/etc/_config.php": "<?php // an old backup", "app/etc/config.php": "<?php // somebody's newer one",
		"app/etc/env.php": "<?php // env",
	} {
		if got, _ := os.ReadFile(filepath.Join(here(), name)); string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if got := len(commandsRun(fake)); got != 0 {
		t.Errorf("%d commands ran, want none", got)
	}
}

// TestInstallStopsWhenSetupInstallFails: set -e, with setup:install's own status. Nothing after
// it runs, and the project's config.php is put back (where the shell left it set aside).
func TestInstallStopsWhenSetupInstallFails(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if len(command) > 2 && command[2] == "setup:install" {
			return outcome{status: 7}, true
		}

		return outcome{}, false
	}

	code, stdout, _ := runInstall(t, false)
	if code != 7 {
		t.Fatalf("install = %d, want setup:install's 7", code)
	}

	if got := len(commandsRun(fake)); got != 1 {
		t.Errorf("%d commands ran, want only setup:install", got)
	}

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}

	// The project's config.php is put back even so, exactly once: no backup is left behind for a
	// later install to mistake for the project's
	if config, _ := os.ReadFile(filepath.Join(here(), "app/etc/config.php")); string(config) != "<?php return ['modules' => []];" {
		t.Errorf("config.php = %q, want the project's own back", config)
	}

	if _, err := os.Stat(filepath.Join(here(), "app/etc/_config.php")); !os.IsNotExist(err) {
		t.Errorf("the backup is still there: %v", err)
	}

	if _, err := os.Stat(filepath.Join(here(), "app/etc/env.php")); !os.IsNotExist(err) {
		t.Errorf("env.php: %v, want it removed", err)
	}
}

// TestInstallStopsWhenTheCacheSettingFails: the second command's status too.
func TestInstallStopsWhenTheCacheSettingFails(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if len(command) > 2 && command[2] == "config:set" {
			return outcome{status: 3}, true
		}

		return outcome{}, false
	}

	if code, _, _ := runInstall(t, false); code != 3 {
		t.Fatalf("install = %d, want config:set's 3", code)
	}

	if got := len(commandsRun(fake)); got != 2 {
		t.Errorf("%d commands ran, want two", got)
	}
}

// TestInstallSearchEngineFollowsTheRelease: a 2.3 lock names no search engine, 2.4.5 names
// Elasticsearch only.
func TestInstallSearchEngineFollowsTheRelease(t *testing.T) {
	for version, equivalent := range map[string]string{"2.3.7-p4": "2.3.7", "2.4.5": "2.4.5"} {
		t.Run(version, func(t *testing.T) {
			fake := installLab(t)
			writeFile(t, "composer.lock", `{"packages": [{"name": "magento/product-community-edition", "version": "`+version+`"}]}`)

			if code, _, stderr := runInstall(t, false); code != exitOK {
				t.Fatalf("install = %d, stderr %q", code, stderr)
			}

			command, password := installedWith(t, fake)
			want := append([]string{"php", "./bin/magento", "setup:install"},
				core.InstallArguments("redis", version, equivalent, "shop.test")...)
			want = append(want, defaultSettingArguments(password)...)

			if diff := cmp.Diff(want, command); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// TestInstallUsesValkeyWhenTheProjectHasIt: the cache host is the service the project declares.
func TestInstallUsesValkeyWhenTheProjectHasIt(t *testing.T) {
	fake := installLab(t)
	fake.description.Services = append(fake.description.Services, core.DescribedService{Name: "valkey"})

	if code, _, _ := runInstall(t, false); code != exitOK {
		t.Fatalf("install = %d", code)
	}

	command, _ := installedWith(t, fake)
	if !contains(command, "--session-save-redis-host=valkey") || !contains(command, "--page-cache-redis-server=valkey") {
		t.Errorf("valkey not used: %q", command)
	}
}

// TestInstallTakesTheVersionFromTheProperties: MAGENTO_VERSION and EQUIVALENT_VERSION, as the
// shell read them from properties.json, win over composer.lock and are not asked for.
func TestInstallTakesTheVersionFromTheProperties(t *testing.T) {
	fake := installLab(t)
	fake.properties = map[string]string{"MAGENTO_VERSION": "2.4.5", "EQUIVALENT_VERSION": "2.4.6"}

	if code, _, _ := runInstall(t, false); code != exitOK {
		t.Fatalf("install = %d", code)
	}

	command, _ := installedWith(t, fake)
	if !contains(command, "--elasticsearch-host=search") || !contains(command, "--opensearch-host=search") {
		t.Errorf("the equivalent 2.4.6 should bring OpenSearch: %q", command)
	}
}

// TestInstallAsksForTheVersionWithoutALock: the latest supported version is offered, and taken
// when nobody answers.
func TestInstallAsksForTheVersionWithoutALock(t *testing.T) {
	fake := installLab(t)
	writeFile(t, "composer.lock", "{}")
	asked := installAnswers(t, "", "", "", "", "", "", "", "", "")

	if code, _, stderr := runInstall(t, false); code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	if len(*asked) == 0 || (*asked)[0] != "Magento version: {2.4.7-p1}" {
		t.Fatalf("asked %q, want the version question first, offering 2.4.7-p1", *asked)
	}

	command, _ := installedWith(t, fake)
	if !contains(command, "--opensearch-host=search") {
		t.Errorf("2.4.7-p1's equivalent 2.4.7 should bring OpenSearch: %q", command)
	}
}

// TestInstallOffersTheTableForAnUnsupportedVersion: a version the tool has no images for is
// said, the table shown, and another read until one is supported.
func TestInstallOffersTheTableForAnUnsupportedVersion(t *testing.T) {
	fake := installLab(t)
	fake.compatibility = "TABLE\n"
	writeFile(t, "composer.lock", "{}")
	installAnswers(t, "9.9.9")
	readVersions(t, "", "2.3.7")

	code, stdout, stderr := runInstall(t, false)
	if code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	refusedOnce := "\nWe don't have support for the version 9.9.9 " +
		"\nPlease, write any version between all versions supported or press Ctrl - C to exit" + "TABLE\n"
	refusedTwice := "\nWe don't have support for the version  " +
		"\nPlease, write any version between all versions supported or press Ctrl - C to exit" + "TABLE\n"
	if !strings.HasPrefix(stdout, refusedOnce+refusedTwice) {
		t.Errorf("stdout = %q", stdout)
	}

	command, _ := installedWith(t, fake)
	if contains(command, "--elasticsearch-host=search") {
		t.Errorf("2.3.7 names no search engine: %q", command)
	}
}

// TestInstallCannotOfferAVersionFromAnEmptyTable: with nobody to ask and no supported release to
// offer, the question is refused, as the shell's was when jq offered "null".
func TestInstallCannotOfferAVersionFromAnEmptyTable(t *testing.T) {
	fake := installLab(t)
	fake.equivalents = map[string]string{}
	writeFile(t, "composer.lock", "{}")

	code, _, stderr := runInstall(t, true)
	if code != exitUsage {
		t.Fatalf("install = %d, want %d", code, exitUsage)
	}

	assertError(t, stderr, "input_required", "Non-interactive mode cannot answer: Magento version:",
		"Pass the value as an option, or run without --yes")

	if got := len(commandsRun(fake)); got != 0 {
		t.Errorf("%d commands ran", got)
	}
}

// TestInstallRefusesAnUnsupportedVersionWithNobodyToAsk: get_equivalent_version_if_exit's
// refusal, input_required with the table as the way out. The question offers only supported
// releases with nobody there, so it is reached here directly.
func TestInstallRefusesAnUnsupportedVersionWithNobodyToAsk(t *testing.T) {
	fake := installLab(t)

	_, _, err := installer{engine: fake}.supported(fake.equivalents, "9.9.9")

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "input_required" || refusal.Code != exitUsage ||
		refusal.Message != "Unsupported Magento version: 9.9.9" ||
		refusal.Hint != "Run 'hm compatibility' and pass a supported version" {
		t.Errorf("supported(9.9.9) = %#v", err)
	}
}

// TestInstallAsksForTheDomainWhenTheProjectHasNone: offered as <project>.local, and lowercased.
func TestInstallAsksForTheDomainWhenTheProjectHasNone(t *testing.T) {
	fake := installLab(t)
	fake.project.Domain = ""
	asked := installAnswers(t, "My.Shop.TEST")

	if code, _, stderr := runInstall(t, false, "-u"); code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	if diff := cmp.Diff([]string{"Define domain {shop.local}"}, *asked); diff != "" {
		t.Errorf("asked (-want +got):\n%s", diff)
	}

	command, _ := installedWith(t, fake)
	if !contains(command, "--base-url=https://my.shop.test/") {
		t.Errorf("base url: %q", command)
	}
}

// TestInstallAsksForEachSetting: in order, offering what the file holds, and every answer is
// written back to the file and used.
func TestInstallAsksForEachSetting(t *testing.T) {
	fake := installLab(t)
	asked := installAnswers(t, "en_US", "", "", "Ada", "", "", "ada")

	code, stdout, stderr := runInstall(t, false)
	if code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	want := []string{"Define language {es_ES}", "Define currency {EUR}", "Define timezone {Europe/Madrid}",
		"Define admin-firstname {hiberus}", "Define admin-lastname {hiberus}",
		"Define admin-email {noreply@hiberus.com}", "Define admin-user {hiberus}"}
	if diff := cmp.Diff(want, *asked); diff != "" {
		t.Errorf("asked (-want +got):\n%s", diff)
	}

	rendered := core.RenderInstallSettings(fake.settings)
	for _, saved := range []string{`"language": "en_US"`, `"admin-firstname": "Ada"`, `"admin-user": "ada"`, `"admin-password": ""`} {
		if !strings.Contains(rendered, saved) {
			t.Errorf("%s not saved in\n%s", saved, rendered)
		}
	}

	command, password := installedWith(t, fake)
	if !contains(command, "--language=en_US") || !contains(command, "--admin-user=ada") {
		t.Errorf("answers not used: %q", command)
	}

	if !strings.Contains(stdout, summary("ada", password)) {
		t.Errorf("the summary should name the user answered: %q", stdout)
	}
}

// TestInstallUseDefaultAsksNothing: -u, and its long form, skip the settings questions.
func TestInstallUseDefaultAsksNothing(t *testing.T) {
	for _, flag := range []string{"-u", "--use-default"} {
		t.Run(flag, func(t *testing.T) {
			installLab(t)
			asked := installAnswers(t)

			if code, _, stderr := runInstall(t, false, flag); code != exitOK {
				t.Fatalf("install = %d, stderr %q", code, stderr)
			}

			if len(*asked) != 0 {
				t.Errorf("asked %q", *asked)
			}
		})
	}
}

// TestInstallCannotGuessASettingWithNoValue: with nobody to ask, a setting the file leaves empty
// is refused rather than invented.
func TestInstallCannotGuessASettingWithNoValue(t *testing.T) {
	fake := installLab(t)
	fake.settings, _ = core.ReadInstallSettings(`{"currency": "EUR"}`)

	code, _, stderr := runInstall(t, true)
	if code != exitUsage {
		t.Fatalf("install = %d, want %d", code, exitUsage)
	}

	assertError(t, stderr, "input_required", "Non-interactive mode cannot answer: Define language",
		"Pass the value as an option, or run without --yes")
}

// TestInstallRespectsAConfiguredPassword: a password ~/.hm/config.json sets is somebody's decision,
// used in its place and not generated.
func TestInstallRespectsAConfiguredPassword(t *testing.T) {
	fake := installLab(t)
	fake.settings, _ = core.ReadInstallSettings(`{"admin-password": "Chosen123", "admin-user": "me"}`)

	if code, stdout, _ := runInstall(t, false, "-u"); code != exitOK || !strings.Contains(stdout, summary("me", "Chosen123")) {
		t.Fatalf("install = %d, stdout %q", code, stdout)
	}

	command, _ := installedWith(t, fake)
	tail := command[len(command)-2:]
	if diff := cmp.Diff([]string{"--admin-password=Chosen123", "--admin-user=me"}, tail); diff != "" {
		t.Errorf("the password keeps its place (-want +got):\n%s", diff)
	}
}

// TestInstallRegistersTheSecondFactor: with the module on, a secret is registered for the admin
// and everything needed to enter it is printed, the QR drawn by the project's own library.
func TestInstallRegistersTheSecondFactor(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		switch {
		case cmp.Equal(command, twoFactorStatus):
			return outcome{captured: "Module is enabled\n"}, true
		case command[1] == "-r":
			return outcome{captured: "##QR##\n"}, true
		}

		return outcome{}, false
	}

	code, stdout, stderr := runInstall(t, false)
	if code != exitOK {
		t.Fatalf("install = %d, stderr %q", code, stderr)
	}

	run := commandsRun(fake)
	if len(run) != 5 {
		t.Fatalf("commands: %q", run)
	}

	secret := run[3][len(run[3])-1]
	if !regexp.MustCompile(`^[A-Z2-7]{32}$`).MatchString(secret) {
		t.Fatalf("secret %q has the wrong shape", secret)
	}

	uri := "otpauth://totp/shop.test:hiberus?secret=" + secret + "&issuer=shop.test"
	want := [][]string{
		{"Capture", "phpfpm", "php", "./bin/magento", "security:tfa:google:set-secret", "hiberus", secret},
		{"Capture", "phpfpm", "php", "-r", qrScript, "--", uri},
	}

	if diff := cmp.Diff(want, run[3:]); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}

	_, password := installedWith(t, fake)
	wantOut := summary("hiberus", password) +
		"\nScan this with your authenticator app:\n\n##QR##\n\n" +
		"  account    shop.test:hiberus\n" +
		"  key        " + secret + "\n" +
		"  type       time based (TOTP)\n\n" +
		"  uri        " + uri + "\n\n"
	if stdout != wantOut {
		t.Errorf("stdout:\n%q\nwant:\n%q", stdout, wantOut)
	}
}

// TestInstallTakesADisabledModuleForDisabled: "Module is disabled" contains "module is", and still
// no secret is registered for it.
func TestInstallTakesADisabledModuleForDisabled(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if cmp.Equal(command, twoFactorStatus) {
			return outcome{captured: "Module is disabled\n"}, true
		}

		return outcome{}, false
	}

	code, stdout, _ := runInstall(t, false)
	if code != exitOK || !strings.HasPrefix(stdout, "Two factor authentication is disabled in this project") {
		t.Fatalf("install = %d, stdout %q", code, stdout)
	}

	if got := len(commandsRun(fake)); got != 3 {
		t.Errorf("%d commands, want no secret registered: %q", got, commandsRun(fake))
	}
}

// TestInstallSaysWhenTheQRCannotBeDrawn: the key is still printed to type in.
func TestInstallSaysWhenTheQRCannotBeDrawn(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		switch {
		case cmp.Equal(command, twoFactorStatus):
			return outcome{captured: "Module is enabled"}, true
		case command[1] == "-r":
			return outcome{status: 1}, true
		}

		return outcome{}, false
	}

	_, stdout, _ := runInstall(t, false)
	if !strings.Contains(stdout, "Scan this with your authenticator app:\n\n  The code could not be drawn. Enter it by hand instead.\n\n  account    ") {
		t.Errorf("stdout = %q", stdout)
	}
}

// TestInstallWarnsWhenTheSecondFactorCannotBeRegistered: a warning, the summary, and no code to
// scan.
func TestInstallWarnsWhenTheSecondFactorCannotBeRegistered(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		switch {
		case cmp.Equal(command, twoFactorStatus):
			return outcome{captured: "module is ENABLED"}, true
		case len(command) > 2 && command[2] == "security:tfa:google:set-secret":
			return outcome{status: 1}, true
		}

		return outcome{}, false
	}

	code, stdout, _ := runInstall(t, false)
	if code != exitOK {
		t.Fatalf("install = %d", code)
	}

	_, password := installedWith(t, fake)
	if want := "Could not register the second factor for 'hiberus'\n" + summary("hiberus", password) + "\n"; stdout != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", stdout, want)
	}

	if got := len(commandsRun(fake)); got != 4 {
		t.Errorf("%d commands, want no QR drawn", got)
	}
}

// TestInstallUnderJSONKeepsDecorationOffStdout: the shell's print_* went to stderr under --json,
// the summary's own lines stayed on stdout, and there was no document.
func TestInstallUnderJSONKeepsDecorationOffStdout(t *testing.T) {
	fake := installLab(t)

	code, stdout, stderr := runInstall(t, true)
	if code != exitOK {
		t.Fatalf("install = %d", code)
	}

	_, password := installedWith(t, fake)

	wantErr := "Two factor authentication is disabled in this project, so none was set up.\n" +
		"Your environment is ready\n" + "This password is not stored anywhere. Save it now.\n"
	if stderr != wantErr {
		t.Errorf("stderr:\n%q\nwant:\n%q", stderr, wantErr)
	}

	wantOut := "\n\n  storefront   https://shop.test/\n  admin        https://shop.test/admin\n\n" +
		"  user         hiberus\n  password     " + password + "\n\n\n"
	if stdout != wantOut {
		t.Errorf("stdout:\n%q\nwant:\n%q", stdout, wantOut)
	}
}

// TestInstallRefusals: each refusal said before anything runs, with its type and code.
func TestInstallRefusals(t *testing.T) {
	t.Run("an option it does not know", func(t *testing.T) {
		fake := installLab(t)

		code, _, stderr := runInstall(t, true, "-x")
		if code != exitUsage {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "invalid_argument", "The command is not correct", "hm install [-u|--use-default]")

		if got := len(commandsRun(fake)); got != 0 {
			t.Errorf("%d commands ran", got)
		}
	})

	// bin/run refused before the command read its options, and in this order
	t.Run("an option it does not know, outside a project", func(t *testing.T) {
		fake := installLab(t)
		fake.project = core.Project{}

		if code, _, _ := runInstall(t, true, "-x"); code != exitProject {
			t.Fatalf("install -x = %d, want the project's refusal first", code)
		}
	})

	t.Run("outside a project", func(t *testing.T) {
		fake := installLab(t)
		fake.project = core.Project{}

		code, _, stderr := runInstall(t, true)
		if code != exitProject {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "project_not_configured",
			"This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid",
			"hm setup")
	})

	t.Run("from a worktree with no environment", func(t *testing.T) {
		fake := installLab(t)
		fake.project.InWorktree = true

		code, _, stderr := runInstall(t, true)
		if code != exitBlocked {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "blocked_in_worktree",
			"'install' is refused from a worktree: it would recreate or destroy the environment of "+fake.project.Root+" with the mounts of this worktree",
			"Run it from "+fake.project.Root+", or repeat with --force")

		t.Setenv("HM_FORCE", "1")
		if code, _, _ := runInstall(t, true); code == exitBlocked {
			t.Error("--force must let it through")
		}
	})

	t.Run("with Docker unreachable", func(t *testing.T) {
		fake := installLab(t)
		fake.dockerDown = true

		// Before the options and before the project, as bin/run checked it
		fake.project = core.Project{}

		code, _, stderr := runInstall(t, true, "-x")
		if code != exitDocker {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "docker_unavailable", "Docker is not running", "Start Docker and try again")
	})

	t.Run("when the project cannot be described", func(t *testing.T) {
		fake := installLab(t)
		fake.outcomes = []outcome{{}, {}, {}, {err: core.Refusal{Kind: "compose_invalid", Code: exitProject,
			Message: "broken", Hint: "fix it"}}}

		code, _, stderr := runInstall(t, true)
		if code != exitProject {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "compose_invalid", "broken", "fix it")
	})

	t.Run("with phpfpm stopped", func(t *testing.T) {
		fake := installLab(t)
		fake.description.Services = []core.DescribedService{{Name: "phpfpm", State: "exited"}}

		code, _, stderr := runInstall(t, true)
		if code != exitService {
			t.Fatalf("install = %d", code)
		}

		assertError(t, stderr, "service_not_running", "Service 'phpfpm' is not running", "hm start phpfpm")

		if got := len(commandsRun(fake)); got != 0 {
			t.Errorf("%d commands ran", got)
		}
	})

	t.Run("with ~/.hm/config.json unreadable", func(t *testing.T) {
		fake := installLab(t)
		fake.outcomes = make([]outcome, 10)
		for at := range fake.outcomes {
			fake.outcomes[at] = outcome{}
		}

		// InstallSettings is the seventh thing asked: Resolve, DockerReachable, Resolve, Describe and
		// the two properties first
		fake.outcomes[6] = outcome{err: errors.New("unexpected end of JSON input")}

		code, _, stderr := runInstall(t, true, "-u")
		if code != exitError {
			t.Fatalf("install = %d, stderr %q", code, stderr)
		}

		assertError(t, stderr, "data_unreadable", "unexpected end of JSON input", "")
	})
}

// assertError checks the JSON error document on stderr.
func assertError(t *testing.T, stderr, kind, message, hint string) {
	t.Helper()

	var document struct {
		OK    bool `json:"ok"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr), &document); err != nil {
		t.Fatalf("stderr is not a document: %q", stderr)
	}

	if document.OK || document.Error.Type != kind || document.Error.Message != message || document.Error.Hint != hint {
		t.Errorf("error = %+v, want %s / %q / %q", document.Error, kind, message, hint)
	}
}

// installAnswers makes the session interactive and answers install's questions in order,
// recording each with the default it offered.
func installAnswers(t *testing.T, answers ...string) *[]string {
	t.Helper()

	t.Setenv("HM_NON_INTERACTIVE", "")

	asked := &[]string{}
	previous := askInstall
	t.Cleanup(func() { askInstall = previous })

	askInstall = func(text, suggestion string) (string, error) {
		*asked = append(*asked, text+" {"+suggestion+"}")

		answer := ""
		if len(answers) > 0 {
			answer, answers = answers[0], answers[1:]
		}

		if answer == "" {
			return suggestion, nil
		}

		return answer, nil
	}

	return asked
}

// readVersions answers the bare reads that follow an unsupported version.
func readVersions(t *testing.T, versions ...string) {
	t.Helper()

	previous := readVersion
	t.Cleanup(func() { readVersion = previous })

	readVersion = func() (string, error) {
		if len(versions) == 0 {
			return "", io.EOF
		}

		version := versions[0]
		versions = versions[1:]

		return version, nil
	}
}

// TestInstallIsAnsweredByTheBinary: the router hands install to this handler rather than to the
// shell implementation.
func TestInstallIsAnsweredByTheBinary(t *testing.T) {
	installLab(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "install", "-x"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("Run(install -x) = %d, stderr %q", code, stderr.String())
	}

	assertError(t, stderr.String(), "invalid_argument", "The command is not correct", "hm install [-u|--use-default]")
}

// TestAFailedInstallWhoseConfigCannotBePutBackSaysSo: setup:install failed and the project's
// config.php could not be put back either — it is still app/etc/_config.php, and that is said
// rather than left for somebody to discover.
func TestAFailedInstallWhoseConfigCannotBePutBackSaysSo(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if len(command) > 2 && command[2] == "setup:install" {
			// A non-empty directory where config.php goes: the rename back cannot replace it
			writeFile(t, "app/etc/config.php/blocker", "x")

			return outcome{status: 7}, true
		}

		return outcome{}, false
	}

	code, _, stderr := runInstall(t, false)
	if code != 7 {
		t.Fatalf("install = %d, want setup:install's 7", code)
	}

	if !strings.Contains(stderr, "_config.php") || !strings.Contains(stderr, "could not be put back") {
		t.Errorf("stderr = %q, want it to say config.php could not be put back and where it is", stderr)
	}

	if _, err := os.Stat(filepath.Join(here(), "app/etc/_config.php")); err != nil {
		t.Errorf("the project's config.php is gone: %v", err)
	}
}

// TestASuccessfulInstallWhoseConfigCannotBePutBackFails: the same, after setup:install worked.
func TestASuccessfulInstallWhoseConfigCannotBePutBackFails(t *testing.T) {
	fake := installLab(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if len(command) > 2 && command[2] == "setup:install" {
			writeFile(t, "app/etc/config.php/blocker", "x")
		}

		return outcome{}, false
	}

	code, _, stderr := runInstall(t, false)
	if code == exitOK || !strings.Contains(stderr, "could not be put back") {
		t.Fatalf("install = %d, stderr %q, want a failure saying config.php could not be put back", code, stderr)
	}
}
