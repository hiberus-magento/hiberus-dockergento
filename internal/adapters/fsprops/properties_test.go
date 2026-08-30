package fsprops

import (
	"os"
	"path/filepath"
	"testing"
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
		t.Fatalf("el proyecto manda: %q", properties["MAGENTO_DIR"])
	}

	if properties["WORKDIR_PHP"] != "/var/www/html" {
		t.Fatalf("lo que el proyecto no dice sale de los valores por defecto: %q", properties["WORKDIR_PHP"])
	}

	if properties["COMPOSE_PROJECT_NAME"] != "shop" {
		t.Fatalf("nombre inesperado: %q", properties["COMPOSE_PROJECT_NAME"])
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
		t.Fatalf("no debería parecer un proyecto: %v", properties)
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
		t.Fatalf("dominio inesperado: %q", properties["DOMAIN"])
	}

	if _, ok := properties["USE_PROXY"]; ok {
		t.Fatal("un booleano no es una property")
	}
}

func TestBrokenJSONIsAnErrorAndNotAnEmptyProject(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "config", "docker", "properties.json"), `{no es json`)

	if _, err := (Reader{}).Load(root); err == nil {
		t.Fatal("debería decir que no puede leerlo")
	}
}
