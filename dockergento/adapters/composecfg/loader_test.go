package composecfg

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The library has to agree with the command, or every read path in the tool changes meaning at
// once. So this is not a test of our conversion: it runs `docker compose config` on the same
// files and compares.

func labFor(t testing.TB) string {
	t.Helper()

	root := t.TempDir()

	compose := `services:
  phpfpm:
    image: hiberusmagento/php:8.3-bookworm
  nginx:
    image: hiberusmagento/nginx:1.28
    ports:
      - 8099:8080
  db:
    image: mariadb:10.6
    ports:
      - 33099:3306
    environment:
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
volumes:
  dbdata:
`
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	overlay := `services:
  phpfpm:
    volumes:
      - ./.:/var/www/html
`
	if err := os.WriteFile(filepath.Join(root, "docker-compose.dev.mac.yml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestTheConfigurationIsWhatComposeSaysItIs(t *testing.T) {
	root := labFor(t)

	loaded, err := Loader{}.Load(root, "lab", []string{"docker-compose.yml", "docker-compose.dev.mac.yml"})
	if err != nil {
		t.Fatalf("no se pudo leer: %v", err)
	}

	command := exec.Command("docker", "compose",
		"-f", filepath.Join(root, "docker-compose.yml"),
		"-f", filepath.Join(root, "docker-compose.dev.mac.yml"),
		"-p", "lab", "config", "--format", "json")
	command.Dir = root

	output, err := command.Output()
	if err != nil {
		t.Skip("docker compose no está disponible aquí")
	}

	var reference struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image string `json:"image"`
			Ports []struct {
				Published string `json:"published"`
				Target    int    `json:"target"`
			} `json:"ports"`
			Environment map[string]*string `json:"environment"`
		} `json:"services"`
	}

	if err := json.Unmarshal(output, &reference); err != nil {
		t.Fatalf("no se pudo leer la referencia: %v", err)
	}

	if loaded.Name != reference.Name {
		t.Fatalf("nombre distinto: %q contra %q", loaded.Name, reference.Name)
	}

	if len(loaded.Services) != len(reference.Services) {
		t.Fatalf("%d servicios contra %d", len(loaded.Services), len(reference.Services))
	}

	for _, service := range loaded.Services {
		expected, ok := reference.Services[service.Name]
		if !ok {
			t.Fatalf("servicio que Compose no ve: %s", service.Name)
		}

		if service.Image != expected.Image {
			t.Fatalf("%s: imagen %q contra %q", service.Name, service.Image, expected.Image)
		}

		if len(service.Ports) != len(expected.Ports) {
			t.Fatalf("%s: %d puertos contra %d", service.Name, len(service.Ports), len(expected.Ports))
		}

		for at, port := range service.Ports {
			if port.Published != expected.Ports[at].Published {
				t.Fatalf("%s: publicado %q contra %q", service.Name, port.Published, expected.Ports[at].Published)
			}
		}

		for key, value := range expected.Environment {
			if value == nil {
				continue
			}

			if service.Environment[key] != *value {
				t.Fatalf("%s: %s=%q contra %q", service.Name, key, service.Environment[key], *value)
			}
		}
	}
}

// A machine overlay that is not there is the other platform's, and skipping it is what the shell
// implementation does too.
func TestAMissingFileIsSkippedRatherThanFatal(t *testing.T) {
	root := labFor(t)

	loaded, err := Loader{}.Load(root, "lab",
		[]string{"docker-compose.yml", "docker-compose.dev.linux.yml"})
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	if len(loaded.Services) != 3 {
		t.Fatalf("servicios inesperados: %d", len(loaded.Services))
	}
}

func TestNoFilesAtAllIsAnError(t *testing.T) {
	if _, err := (Loader{}).Load(t.TempDir(), "lab", []string{"no-existe.yml"}); err == nil {
		t.Fatal("sin ficheros no hay proyecto")
	}
}

// The order cannot depend on which key the runtime walked first.
func TestServicesComeOutInTheSameOrderEveryTime(t *testing.T) {
	root := labFor(t)

	for attempt := 0; attempt < 5; attempt++ {
		loaded, err := Loader{}.Load(root, "lab", []string{"docker-compose.yml"})
		if err != nil {
			t.Fatal(err)
		}

		if loaded.Services[0].Name != "db" || loaded.Services[2].Name != "phpfpm" {
			t.Fatalf("orden inesperado: %s, %s", loaded.Services[0].Name, loaded.Services[2].Name)
		}
	}
}
