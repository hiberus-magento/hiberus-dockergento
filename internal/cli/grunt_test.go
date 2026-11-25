package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// grunt prepares a theme's front-end tooling in the php container and compiles it: three sample
// files put in place when missing, npm install and update, the theme written into Magento's grunt
// configuration, and grunt's own exec and watch. Each step is one `bash -c` in the container, as
// the shell implementation ran them, and the first that fails stops the rest, as `set -e` did.
//

func gruntSteps(theme, locale string) []string {
	return []string{
		`if [ -f "package.json.sample" ] && [ ! -f "package.json" ]; then cp package.json.sample package.json; fi`,
		`if [ -f "grunt-config.json.sample" ] && [ ! -f "grunt-config.json" ]; then cp grunt-config.json.sample grunt-config.json; fi`,
		`if [ -f "Gruntfile.js.sample" ] && [ ! -f "Gruntfile.js" ]; then cp Gruntfile.js.sample Gruntfile.js; fi`,
		`npm install && npm update`,
		`if [ -d "dev/tools/grunt/configs" ]; then echo "module.exports = {magento: {area: 'frontend', name: '` + theme +
			`', locale: '` + locale + `', files: ['css/styles-m', 'css/styles-l'], dsl: 'less' }};" > dev/tools/grunt/configs/local-themes.js; fi`,
		`grunt exec:magento && grunt watch:magento`,
	}
}

func execCommands(calls []call) []string {
	commands := []string{}

	for _, c := range calls {
		if c.Method == "Exec" {
			commands = append(commands, strings.Join(c.Command[2:], " "))
		}
	}

	return commands
}

func TestGruntRunsItsStepsInOrderInThePHPContainer(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	if code := grunt([]string{"Vendor/theme", "es_ES"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("grunt = %d, want exitOK", code)
	}

	want := gruntSteps("Vendor/theme", "es_ES")
	got := execCommands(fake.calls)

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ran:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	for _, c := range fake.calls {
		if c.Method == "Exec" && (c.Service != phpService || c.Command[0] != "bash" || c.Command[1] != "-c") {
			t.Fatalf("a step ran as %+v, want bash -c in %s", c, phpService)
		}
	}
}

// TestAFailingGruntStepStopsTheRest: npm failing means there is nothing for grunt to compile.
func TestAFailingGruntStepStopsTheRest(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	// Resolve, three sample copies, then npm fails
	fake.outcomes = []outcome{{}, {}, {}, {}, {status: 17}}

	if code := grunt([]string{"Vendor/theme", "es_ES"}, io.Discard, io.Discard, false); code != 17 {
		t.Fatalf("grunt with npm failing = %d, want npm's own 17", code)
	}

	if got := execCommands(fake.calls); len(got) != 4 {
		t.Fatalf("ran %d steps, want it to stop at npm (4): %v", len(got), got)
	}
}

// TestGruntNeedsAThemeAndALocale: anything else is a usage error before anything runs. The shell
// implementation printed the usage and exited 0; a script asking whether it worked was told yes.
func TestGruntNeedsAThemeAndALocale(t *testing.T) {
	for _, args := range [][]string{nil, {"Vendor/theme"}, {"theme", "es_ES"}, {"Vendor/theme", "es_ES", "extra"}} {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		if code := grunt(args, io.Discard, io.Discard, true); code != exitUsage {
			t.Fatalf("grunt %v = %d, want exitUsage", args, code)
		}

		if len(fake.calls) != 0 {
			t.Fatalf("grunt %v asked the engine %v, want nothing", args, fake.calls)
		}
	}
}

// TestGruntRefusesAThemeOrLocaleOfAnotherShape: both are written into a `bash -c` in the
// container, so anything not shaped like a Magento theme or locale is a usage error before
// anything runs, whatever it would have done to that shell.
func TestGruntRefusesAThemeOrLocaleOfAnotherShape(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"a quote in the theme", []string{"Vendor/th'eme", "es_ES"}, "Not a Magento theme: Vendor/th'eme"},
		{"a command in the theme", []string{"Vendor/theme;id", "es_ES"}, "Not a Magento theme: Vendor/theme;id"},
		{"a theme with two slashes", []string{"Vendor/theme/x", "es_ES"}, "Not a Magento theme: Vendor/theme/x"},
		{"a theme with no vendor", []string{"/theme", "es_ES"}, "Not a Magento theme: /theme"},
		{"a command in the locale", []string{"Vendor/theme", "es_ES'; id; '"}, "Not a locale: es_ES'; id; '"},
		{"a language with no country", []string{"Vendor/theme", "es"}, "Not a locale: es"},
		{"a locale in the wrong case", []string{"Vendor/theme", "ES_es"}, "Not a locale: ES_es"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			t.Setenv("COMMAND_BIN_NAME", "hm")

			var stdout, stderr strings.Builder
			args := append([]string{"--json", "grunt"}, tt.args...)

			if code := Run(args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("grunt %v = %d, want exitUsage", tt.args, code)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}

			assertError(t, stderr.String(), "invalid_argument", tt.message, "hm grunt <Vendor/theme> <locale_LOCALE>")

			if len(fake.calls) != 0 {
				t.Errorf("grunt %v asked the engine %v, want nothing", tt.args, fake.calls)
			}
		})
	}
}

// TestGruntAcceptsTheShapesMagentoUses: a theme name with a hyphen or an underscore, and a locale
// that names its script, are Magento's own and run as before.
func TestGruntAcceptsTheShapesMagentoUses(t *testing.T) {
	for _, args := range [][]string{{"Magento/luma", "en_US"}, {"My_Vendor/my-theme_2", "zh_Hans_CN"}} {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		if code := grunt(args, io.Discard, io.Discard, false); code != exitOK {
			t.Fatalf("grunt %v = %d, want exitOK", args, code)
		}

		if got, want := execCommands(fake.calls), gruntSteps(args[0], args[1]); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("grunt %v ran:\n%s\nwant:\n%s", args, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

// TestTheRouterAnswersForGruntAndUpdate: both reach their handlers instead of the shell.
func TestTheRouterAnswersForGruntAndUpdate(t *testing.T) {
	for _, args := range [][]string{{"grunt", "Vendor/theme", "es_ES"}, {"update"}} {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		Run(args, io.Discard, io.Discard)

		if len(fake.calls) == 0 {
			t.Fatalf("%v fell through to the shell implementation", args)
		}
	}
}
