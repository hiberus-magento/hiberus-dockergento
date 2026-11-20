package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What create-project does, step by step, without Docker: the steps go through the Legacy door to
// a recorder, the files are written to a temporary directory, and the tool's data is a fixture with
// two releases whose order as text is the wrong one.
//

// onDisk is ports.FS on the real filesystem, under a temporary directory.
type onDisk struct{}

func (onDisk) IsDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

func (onDisk) Exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func (onDisk) Read(path string) string {
	contents, _ := os.ReadFile(path) //nolint:gosec

	return string(contents)
}

func (onDisk) MkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

// memoryProperties keeps what was saved, so that what create-project reads back is what it wrote.
type memoryProperties struct{ values map[string]string }

func (p *memoryProperties) Load(string) (map[string]string, error) {
	copied := map[string]string{}
	for key, value := range p.values {
		copied[key] = value
	}

	return copied, nil
}

func (p *memoryProperties) Set(_, key, value string) error {
	p.values[key] = value

	return nil
}

// stepRecorder is the Legacy door: it records every step, answers with the code given for the step
// at that position, and lets a test look at the host before a step runs.
type stepRecorder struct {
	ran    [][]string
	codes  map[int]int
	before func(step []string)
}

func (r *stepRecorder) Run(step []string) (int, error) {
	if r.before != nil {
		r.before(step)
	}

	r.ran = append(r.ran, step)

	return r.codes[len(r.ran)-1], nil
}

// toolFixture is the tool's own installation: two supported releases, 2.4.9 and 2.4.10, and the
// templates the compose files are written from.
func toolFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"data/equivalent_versions.json": `{"2.4.9": "2.4.9", "2.4.10": "2.4.9", "2.4.7": "2.4.7"}`,
		"data/requirements.json": `{
			"2.4.9": {"php": "8.3-fpm", "mailhog": "hiberusmagento/mailhog:1"},
			"2.4.7": {"php": "8.2-fpm", "mailhog": "hiberusmagento/mailhog:1"}}`,
		"docker-compose/docker-compose.template.yml": "{YML_VERSION}services:\n  phpfpm:\n    image: <php_version>\n" +
			"  <mail_service>:\n    image: <mail_version>\n",
		"docker-compose/docker-compose.dev.mac.template.yml":   "{YML_VERSION}# {MAGENTO_DIR}\n# {FILES_IN_GIT}\n",
		"docker-compose/docker-compose.dev.linux.template.yml": "{YML_VERSION}# {MAGENTO_DIR}\n",
	}

	for name, contents := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func creating(t *testing.T, legacy *stepRecorder, properties map[string]string) Setup {
	t.Helper()

	if properties == nil {
		properties = map[string]string{}
	}

	return Setup{
		Properties: &memoryProperties{values: properties},
		FS:         onDisk{},
		Tooling:    tools{compose: "2.30.0"},
		Legacy:     legacy,
		Assets:     os.DirFS(toolFixture(t)),
		Platform:   "linux",
		Binary:     "hm",
	}
}

// afterTheEnvironment is every step from the install onwards, which is the same for every project
// but for the domain and whether Magento's installation takes its defaults.
func afterTheEnvironment(domain string, install []string) [][]string {
	return [][]string{
		{"restart", "phpfpm"},
		{"start"},
		{"composer", "install"},
		install,
		{"magento", "setup:upgrade"},
		{"magento", "deploy:mode:set", "developer"},
		{"restart"},
		{"ssl", domain},
		{"set-host", domain, "--no-database"},
		{"restart"},
	}
}

func creatingMagento(edition, version, workdir string) [][]string {
	return [][]string{
		{"start"},
		{"exec", "sh", "-c",
			"[ -d ./var/composer_home ] && cp /var/www/.composer/auth.json ./var/composer_home/auth.json || true"},
		{"exec", "composer", "create-project", "--no-install", "--repository=https://repo.magento.com/",
			"magento/project-" + edition + "-edition=" + version, "/tmp/magento-new"},
		{"exec", "sh", "-c", "cp -R /tmp/magento-new/. " + workdir + "/"},
		{"exec", "rm", "-rf", "/tmp/magento-new"},
		{"copy-from-container", "composer.json"},
	}
}

func TestCreateProjectRunsItsStepsInTheShellImplementationsOrder(t *testing.T) {
	legacy := &stepRecorder{}
	setup := creating(t, legacy, nil)
	dir := t.TempDir()

	options := core.CreateProjectOptions{
		ProjectName: "shop", Domain: "shop.local", Edition: "community", Version: "2.4.7", Root: ".",
	}
	if err := setup.CreateProject(dir, options, false); err != nil {
		t.Fatalf("CreateProject = %v, want no error", err)
	}

	want := append(creatingMagento("community", "2.4.7", "/var/www/html"),
		afterTheEnvironment("shop.local", []string{"install"})...)

	if diff := cmp.Diff(want, legacy.ran); diff != "" {
		t.Errorf("steps run (-want +got):\n%s", diff)
	}

	if compose := (onDisk{}).Read(filepath.Join(dir, "docker-compose.yml")); !strings.Contains(compose, "hiberusmagento/php:8.2-fpm") {
		t.Errorf("docker-compose.yml = %q, want the images of 2.4.7", compose)
	}
}

// TestCreateProjectWithDefaults: -u takes the latest release by release order, community, the
// directory's own name, and installs Magento with its defaults too.
func TestCreateProjectWithDefaults(t *testing.T) {
	legacy := &stepRecorder{}
	setup := creating(t, legacy, nil)
	dir := filepath.Join(t.TempDir(), "MiTienda")

	options, err := core.ParseCreateProject([]string{"-u"}, "hm")
	if err != nil {
		t.Fatal(err)
	}

	if err := setup.CreateProject(dir, options, false); err != nil {
		t.Fatalf("CreateProject -u = %v, want no error", err)
	}

	want := append(creatingMagento("community", "2.4.10", "/var/www/html"),
		afterTheEnvironment("mitienda.local", []string{"install", "-u"})...)

	if diff := cmp.Diff(want, legacy.ran); diff != "" {
		t.Errorf("steps run (-want +got):\n%s", diff)
	}
}

func TestTheProjectTemplateIsTheEditionsOwn(t *testing.T) {
	for _, edition := range []string{"community", "enterprise"} {
		t.Run(edition, func(t *testing.T) {
			legacy := &stepRecorder{}
			setup := creating(t, legacy, nil)

			options := core.CreateProjectOptions{ProjectName: "shop", Edition: edition, Version: "2.4.9", Root: "."}
			if err := setup.CreateProject(t.TempDir(), options, false); err != nil {
				t.Fatalf("CreateProject = %v, want no error", err)
			}

			want := []string{"exec", "composer", "create-project", "--no-install",
				"--repository=https://repo.magento.com/", "magento/project-" + edition + "-edition=2.4.9",
				"/tmp/magento-new"}
			if diff := cmp.Diff(want, legacy.ran[2]); diff != "" {
				t.Errorf("composer create-project (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWhatIsMissingIsAsked: the edition is chosen between the two there are, and the version is
// asked with the latest release suggested — 2.4.10, which as text sorts before 2.4.9.
func TestWhatIsMissingIsAsked(t *testing.T) {
	legacy := &stepRecorder{}
	setup := creating(t, legacy, nil)

	var asked, chosen []string
	setup.Ask = func(question, suggestion string) (string, error) {
		asked = append(asked, question+" "+suggestion)

		return "", nil
	}
	setup.Choose = func(question string, options []string) (string, error) {
		chosen = append(chosen, question+" "+strings.Join(options, ","))

		return "enterprise", nil
	}

	if err := setup.CreateProject(t.TempDir(), core.CreateProjectOptions{ProjectName: "shop", Root: "."}, false); err != nil {
		t.Fatalf("CreateProject = %v, want no error", err)
	}

	if diff := cmp.Diff([]string{"Magento edition: community,enterprise"}, chosen); diff != "" {
		t.Errorf("chosen (-want +got):\n%s", diff)
	}

	if len(asked) == 0 || asked[0] != "Magento version: 2.4.10" {
		t.Errorf("asked %v, want the version first, with 2.4.10 suggested", asked)
	}

	if got := legacy.ran[2][5]; got != "magento/project-enterprise-edition=2.4.10" {
		t.Errorf("project template = %q, want the chosen edition at the suggested version", got)
	}
}

// TestAnUnsupportedVersionIsRefusedBeforeAnythingHappens: the same refusal setup gives, and the
// directory is left exactly as it was.
func TestAnUnsupportedVersionIsRefusedBeforeAnythingHappens(t *testing.T) {
	legacy := &stepRecorder{}
	properties := map[string]string{}
	setup := creating(t, legacy, properties)
	dir := t.TempDir()

	options := core.CreateProjectOptions{ProjectName: "shop", Edition: "community", Version: "9.9.9", Root: "src"}
	err := setup.CreateProject(dir, options, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "unsupported_version" || refusal.Code != 2 {
		t.Fatalf("CreateProject 9.9.9 = %v, want an unsupported_version refusal with code 2", err)
	}

	if len(legacy.ran) != 0 {
		t.Errorf("ran %v, want nothing", legacy.ran)
	}

	if len(properties) != 0 {
		t.Errorf("saved %v, want nothing", properties)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the directory has %v, want it untouched", entries)
	}
}

// TestAFailingStepStopsCreateProject: with its own exit code, and nothing after it runs.
func TestAFailingStepStopsCreateProject(t *testing.T) {
	legacy := &stepRecorder{codes: map[int]int{2: 9}}
	setup := creating(t, legacy, nil)

	options := core.CreateProjectOptions{ProjectName: "shop", Edition: "community", Version: "2.4.9", Root: "."}
	err := setup.CreateProject(t.TempDir(), options, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "setup_failed" || refusal.Code != 9 {
		t.Fatalf("CreateProject with a failing step = %v, want a setup_failed refusal with code 9", err)
	}

	if !strings.Contains(refusal.Message, "composer create-project") {
		t.Errorf("message = %q, want it to name the step", refusal.Message)
	}

	if len(legacy.ran) != 3 {
		t.Errorf("ran %v, want nothing after the failed step", legacy.ran)
	}
}

// TestTheComposerFilesAreRealFilesBeforeAnythingStarts: Docker Desktop turns a missing bind-mount
// source into a directory nobody can remove, so both are on the host before the first step; and
// the lock the project template came with is emptied again before the install.
func TestTheComposerFilesAreRealFilesBeforeAnythingStarts(t *testing.T) {
	dir := t.TempDir()
	magento := filepath.Join(dir, "src")
	read := func(name string) string { return (onDisk{}).Read(filepath.Join(magento, name)) }

	var atStart, atRestart []string
	legacy := &stepRecorder{}
	legacy.before = func(step []string) {
		switch strings.Join(step, " ") {
		case "start":
			if atStart == nil {
				atStart = []string{read("composer.json"), read("composer.lock")}
			}
		case "copy-from-container composer.json":
			// What copying the template over the working directory leaves on a bind mount
			os.WriteFile(filepath.Join(magento, "composer.lock"), []byte(`{"packages": []}`), 0o644) //nolint:errcheck
		case "restart phpfpm":
			atRestart = []string{read("composer.lock")}
		}
	}

	setup := creating(t, legacy, nil)

	options := core.CreateProjectOptions{ProjectName: "shop", Edition: "community", Version: "2.4.9", Root: "src"}
	if err := setup.CreateProject(dir, options, false); err != nil {
		t.Fatalf("CreateProject = %v, want no error", err)
	}

	if diff := cmp.Diff([]string{"{}\n", "{}\n"}, atStart); diff != "" {
		t.Errorf("composer.json and composer.lock at the first step (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"{}\n"}, atRestart); diff != "" {
		t.Errorf("composer.lock before the install (-want +got):\n%s", diff)
	}
}

// TestTheProjectIsCopiedOverTheConfiguredWorkdir: WORKDIR_PHP is the project's property, merged
// over the tool's own.
func TestTheProjectIsCopiedOverTheConfiguredWorkdir(t *testing.T) {
	legacy := &stepRecorder{}
	setup := creating(t, legacy, map[string]string{"WORKDIR_PHP": "/app"})

	options := core.CreateProjectOptions{ProjectName: "shop", Edition: "community", Version: "2.4.9", Root: "."}
	if err := setup.CreateProject(t.TempDir(), options, false); err != nil {
		t.Fatalf("CreateProject = %v, want no error", err)
	}

	want := []string{"exec", "sh", "-c", "cp -R /tmp/magento-new/. /app/"}
	if diff := cmp.Diff(want, legacy.ran[3]); diff != "" {
		t.Errorf("copy step (-want +got):\n%s", diff)
	}
}
