package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

//
// The mail catcher a project chooses, rendered from the real template with the images the tool
// uses for Magento 2.4.7. The template and the requirements are both data files somebody edits,
// so the rules are checked against them rather than against a template made up for the test.
//

// composeService is the part of a rendered service these rules look at.
type composeService struct {
	Image     string   `yaml:"image"`
	Ports     []string `yaml:"ports"`
	DependsOn []string `yaml:"depends_on"`
	Networks  struct {
		Default struct {
			Aliases []string `yaml:"aliases"`
		} `yaml:"default"`
	} `yaml:"networks"`
}

// renderedFor renders the real compose template for one mail catcher choice and parses it.
func renderedFor(t *testing.T, mail string) map[string]composeService {
	t.Helper()

	root := filepath.Join("..", "..")

	template, err := os.ReadFile(filepath.Join(root, "docker-compose", "docker-compose.template.yml"))
	if err != nil {
		t.Fatalf("reading the compose template: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "data", "requirements.json"))
	if err != nil {
		t.Fatalf("reading the requirements: %v", err)
	}

	table, err := ReadRequirements(string(contents))
	if err != nil {
		t.Fatalf("ReadRequirements = %v, want the table", err)
	}

	requirements, known := table["2.4.7"]
	if !known {
		t.Fatal("the requirements have no 2.4.7 row, want one")
	}

	written, err := RenderCompose(string(template), requirements, mail)
	if err != nil {
		t.Fatalf("RenderCompose(%q) = %v, want a compose file", mail, err)
	}

	// The real flow fills the version line in afterwards; an empty one is what a current
	// Compose gets
	written = strings.ReplaceAll(written, "{YML_VERSION}", "")

	var parsed struct {
		Services map[string]composeService `yaml:"services"`
	}

	if err := yaml.Unmarshal([]byte(written), &parsed); err != nil {
		t.Fatalf("the compose file rendered for %q is not YAML: %v\n%s", mail, err, written)
	}

	return parsed.Services
}

// The default existed before the choice did, so a project that never chose keeps what it had.
func TestWithNoPreferenceTheMailCatcherIsStillMailhog(t *testing.T) {
	services := renderedFor(t, "")

	mailhog, present := services["mailhog"]
	if !present {
		t.Fatalf("services with no preference = %v, want mailhog among them", reflect.ValueOf(services).MapKeys())
	}

	if _, present := services["mailpit"]; present {
		t.Fatal("services with no preference include mailpit, want only mailhog")
	}

	if mailhog.Image != "hiberusmagento/mailhog:1" {
		t.Fatalf("mailhog image = %q, want hiberusmagento/mailhog:1", mailhog.Image)
	}
}

func TestAskingForMailhogRendersTheSameAsNoPreference(t *testing.T) {
	if implicit, explicit := renderedFor(t, ""), renderedFor(t, "mailhog"); !reflect.DeepEqual(implicit, explicit) {
		t.Fatalf("RenderCompose(mailhog) = %+v, want the same as no preference %+v", explicit, implicit)
	}
}

func TestChoosingMailpitRendersItsOwnServiceAndImage(t *testing.T) {
	services := renderedFor(t, "mailpit")

	mailpit, present := services["mailpit"]
	if !present {
		t.Fatalf("services for mailpit = %v, want mailpit among them", reflect.ValueOf(services).MapKeys())
	}

	if _, present := services["mailhog"]; present {
		t.Fatal("services for mailpit include mailhog, want only mailpit")
	}

	if mailpit.Image != "hiberusmagento/mailpit:1" {
		t.Fatalf("mailpit image = %q, want hiberusmagento/mailpit:1", mailpit.Image)
	}
}

// PHP sends the mail, so it has to start after whichever catcher receives it.
func TestPHPDependsOnTheChosenMailCatcher(t *testing.T) {
	for _, chosen := range []string{"mailhog", "mailpit"} {
		dependsOn := renderedFor(t, chosen)["phpfpm"].DependsOn

		found := false

		for _, service := range dependsOn {
			found = found || service == chosen
		}

		if !found {
			t.Fatalf("phpfpm depends_on for %s = %v, want %s in it", chosen, dependsOn, chosen)
		}
	}
}

// A Magento installed with its SMTP host set to `mailhog` keeps delivering after a switch, without
// touching its configuration, because both catchers answer to that name.
func TestEitherMailCatcherAnswersToTheNameMailhog(t *testing.T) {
	for _, chosen := range []string{"mailhog", "mailpit"} {
		aliases := renderedFor(t, chosen)[chosen].Networks.Default.Aliases

		found := false

		for _, alias := range aliases {
			found = found || alias == "mailhog"
		}

		if !found {
			t.Fatalf("%s network aliases = %v, want mailhog in them", chosen, aliases)
		}
	}
}

// Whatever bookmark or script reaches the mailbox keeps reaching it after a switch.
func TestTheMailboxIsPublishedOnTheSamePortEitherWay(t *testing.T) {
	mailhog := renderedFor(t, "mailhog")["mailhog"].Ports
	mailpit := renderedFor(t, "mailpit")["mailpit"].Ports

	if len(mailhog) == 0 || !reflect.DeepEqual(mailhog, mailpit) {
		t.Fatalf("mailpit ports = %v, want the same as mailhog's %v", mailpit, mailhog)
	}
}
