package fsprops

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The shell implementation merges the tool's own properties with the project's, and a port that
// only read the project's answered differently for every value a project never set — WORKDIR_PHP,
// the compose file names, the mail catcher. It looked right until the two were compared.

func write(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheProjectIsReadOverTheDefaults(t *testing.T) {
	root := t.TempDir()
	defaults := filepath.Join(root, "data", "properties.json")

	write(t, defaults, `{"WORKDIR_PHP": "/var/www/html", "MAGENTO_DIR": ".", "MAIL_SERVICE": "mailhog"}`)
	write(t, filepath.Join(root, "project", "config", "docker", "properties.json"),
		`{"MAGENTO_DIR": "./src", "COMPOSE_PROJECT_NAME": "shop"}`)

	properties, err := (Reader{Defaults: defaults}).Load(filepath.Join(root, "project"))
	if err != nil {
		t.Fatal(err)
	}

	if properties["MAGENTO_DIR"] != "./src" {
		t.Fatalf("MAGENTO_DIR = %q, want the project's over the default", properties["MAGENTO_DIR"])
	}

	if properties["WORKDIR_PHP"] != "/var/www/html" {
		t.Fatalf("WORKDIR_PHP = %q, want the default where the project says nothing", properties["WORKDIR_PHP"])
	}

	if properties["COMPOSE_PROJECT_NAME"] != "shop" {
		t.Fatalf("COMPOSE_PROJECT_NAME = %q, want the project's", properties["COMPOSE_PROJECT_NAME"])
	}
}

// A directory with no properties of its own is a directory that is not a project, and it must not
// look configured just because the tool has defaults.
func TestADirectoryWithNoPropertiesIsNotAProject(t *testing.T) {
	root := t.TempDir()
	defaults := filepath.Join(root, "data", "properties.json")

	write(t, defaults, `{"WORKDIR_PHP": "/var/www/html"}`)

	properties, err := (Reader{Defaults: defaults}).Load(filepath.Join(root, "cualquiera"))
	if err != nil {
		t.Fatal(err)
	}

	if len(properties) != 0 {
		t.Fatalf("properties of a directory that is not a project = %v, want none", properties)
	}
}

// Values are read as strings because that is what they are in the file — and what the shell
// implementation assumes: a boolean there breaks its jq outright.
func TestOnlyStringsAreRead(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties.json"),
		`{"DOMAIN": "shop.test", "USE_PROXY": true, "PORT": 80}`)

	properties, err := (Reader{}).Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if properties["DOMAIN"] != "shop.test" {
		t.Fatalf("DOMAIN = %q, want the one written down", properties["DOMAIN"])
	}

	if _, ok := properties["USE_PROXY"]; ok {
		t.Fatal("a boolean was read as a property, want it left out")
	}
}

func TestBrokenJSONIsAnErrorAndNotAnEmptyProject(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties.json"), `{no es json`)

	if _, err := (Reader{}).Load(root); err == nil {
		t.Fatal("unreadable file = nil, want it said")
	}
}

//
// The format before properties.json: KEY=value lines in config/docker/properties. The shell
// implementation converted it on every invocation; a project that only has it must not look like
// a directory that is not a project.
//

func TestTheOldFormatIsConvertedOnceAndKept(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties"), strings.Join([]string{
		"# written by hand",
		"COMPOSE_PROJECT_NAME=tienda",
		"   DOMAIN=tienda.test",
		"",
		`MAGENTO_DIR="./src"`,
		"EXTRA=a=b=c",
		"EMPTY=",
	}, "\n")+"\n")

	var notices bytes.Buffer

	properties, err := (Reader{Notices: &notices}).Load(root)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"COMPOSE_PROJECT_NAME": "tienda", "DOMAIN": "tienda.test", "MAGENTO_DIR": "./src",
		"EXTRA": "a=b=c", "EMPTY": "",
	}
	if diff := cmp.Diff(want, properties); diff != "" {
		t.Fatalf("properties (-want +got):\n%s", diff)
	}

	written, err := os.ReadFile(filepath.Join(root, "config", "docker", "properties.json"))
	if err != nil {
		t.Fatalf("properties.json = %v, want it written", err)
	}

	var converted map[string]string
	if err := json.Unmarshal(written, &converted); err != nil {
		t.Fatalf("properties.json is not JSON: %v\n%s", err, written)
	}

	if diff := cmp.Diff(want, converted); diff != "" {
		t.Fatalf("properties.json (-want +got):\n%s", diff)
	}

	if _, err := os.Stat(filepath.Join(root, "config", "docker", "properties")); !os.IsNotExist(err) {
		t.Fatalf("the old file is still at its name (%v), want it moved aside", err)
	}

	kept, err := os.ReadFile(filepath.Join(root, "config", "docker", "properties.bak"))
	if err != nil || !strings.Contains(string(kept), "COMPOSE_PROJECT_NAME=tienda") {
		t.Fatalf("properties.bak = %q, %v; want the old file, untouched", kept, err)
	}

	said := notices.String()
	if !strings.Contains(said, "config/docker/properties.json") || !strings.Contains(said, "config/docker/properties.bak") {
		t.Fatalf("announced %q, want the conversion and where the old file is", said)
	}

	// Once: the second load reads properties.json and announces nothing
	notices.Reset()

	if _, err := (Reader{Notices: &notices}).Load(root); err != nil || notices.Len() != 0 {
		t.Fatalf("second load = %v, announced %q; want nothing converted again", err, notices.String())
	}
}

// properties.json wins: a project that has it loads exactly as before, and the old file beside it
// is left where it is.
func TestTheOldFormatIsIgnoredWhenPropertiesJSONExists(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties.json"), `{"COMPOSE_PROJECT_NAME": "nueva"}`)
	write(t, filepath.Join(root, "config", "docker", "properties"), "COMPOSE_PROJECT_NAME=vieja\n")

	var notices bytes.Buffer

	properties, err := (Reader{Notices: &notices}).Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]string{"COMPOSE_PROJECT_NAME": "nueva"}, properties); diff != "" {
		t.Fatalf("properties (-want +got):\n%s", diff)
	}

	if _, err := os.Stat(filepath.Join(root, "config", "docker", "properties")); err != nil {
		t.Fatalf("the old file = %v, want it left alone", err)
	}

	if notices.Len() != 0 {
		t.Fatalf("announced %q, want nothing", notices.String())
	}
}

// A backup already there is somebody's, and is not overwritten: the old file goes to a free name.
func TestAnExistingBackupIsNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties"), "DOMAIN=tienda.test\n")
	write(t, filepath.Join(root, "config", "docker", "properties.bak"), "anterior\n")
	write(t, filepath.Join(root, "config", "docker", "properties.bak.1"), "anterior a la anterior\n")

	var notices bytes.Buffer

	if _, err := (Reader{Notices: &notices}).Load(root); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{
		"properties.bak": "anterior\n", "properties.bak.1": "anterior a la anterior\n",
		"properties.bak.2": "DOMAIN=tienda.test\n",
	} {
		got, err := os.ReadFile(filepath.Join(root, "config", "docker", name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}

	if !strings.Contains(notices.String(), "properties.bak.2") {
		t.Errorf("announced %q, want the name the old file was kept under", notices.String())
	}
}

// Nothing is written into a directory that is not a project.
func TestNothingIsConvertedWhereThereIsNoProject(t *testing.T) {
	root := t.TempDir()

	var notices bytes.Buffer

	if _, err := (Reader{Notices: &notices}).Load(root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "config")); !os.IsNotExist(err) {
		t.Fatalf("config = %v, want nothing created", err)
	}

	if notices.Len() != 0 {
		t.Fatalf("announced %q, want nothing", notices.String())
	}
}
