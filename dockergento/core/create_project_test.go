package core

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What create-project's parser takes: the shell implementation's getopts string ":p:e:v:r:u", and
// setup's long names for the same options.
//

func TestEveryCreateProjectOptionIsTaken(t *testing.T) {
	want := CreateProjectOptions{
		ProjectName: "shop", Domain: "shop.local", Edition: "enterprise", Version: "2.4.7", Root: "src",
	}

	cases := map[string][]string{
		"short forms":               {"-p", "shop", "-e", "enterprise", "-v", "2.4.7", "-r", "src"},
		"long forms with a space":   {"--project-name", "shop", "--edition", "enterprise", "--version", "2.4.7", "--root-directory", "src"},
		"long forms with an equals": {"--project-name=shop", "--edition=enterprise", "--version=2.4.7", "--root-directory=src"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			options, err := ParseCreateProject(args, "hm")
			if err != nil {
				t.Fatalf("ParseCreateProject(%v) = %v, want no error", args, err)
			}

			if diff := cmp.Diff(want, options); diff != "" {
				t.Errorf("options (-want +got):\n%s", diff)
			}
		})
	}
}

// TestNamingTheProjectSuggestsItsDomain: -p set the domain to NAME.local in the shell
// implementation, and nothing else sets the domain.
func TestNamingTheProjectSuggestsItsDomain(t *testing.T) {
	options, err := ParseCreateProject([]string{"-p", "tienda"}, "hm")
	if err != nil {
		t.Fatalf("= %v, want no error", err)
	}

	if options.Domain != "tienda.local" {
		t.Errorf("domain = %q, want tienda.local", options.Domain)
	}
}

// TestUseDefaultFillsWhatTheInstructionCan: community and the project's own directory; the name,
// the domain and the version are left to whoever knows the directory and the table of releases.
// An explicit option wins over the default in either order.
func TestUseDefaultFillsWhatTheInstructionCan(t *testing.T) {
	cases := map[string]struct {
		args []string
		want CreateProjectOptions
	}{
		"alone": {[]string{"-u"},
			CreateProjectOptions{Edition: "community", Root: ".", UseDefault: true}},
		"long form": {[]string{"--use-default"},
			CreateProjectOptions{Edition: "community", Root: ".", UseDefault: true}},
		"explicit options after it": {[]string{"-u", "-e", "enterprise", "-r", "src", "-p", "shop"},
			CreateProjectOptions{ProjectName: "shop", Domain: "shop.local", Edition: "enterprise", Root: "src", UseDefault: true}},
		"explicit options before it": {[]string{"-e", "enterprise", "-v", "2.4.6", "-u"},
			CreateProjectOptions{Edition: "enterprise", Version: "2.4.6", Root: ".", UseDefault: true}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			options, err := ParseCreateProject(tt.args, "hm")
			if err != nil {
				t.Fatalf("ParseCreateProject(%v) = %v, want no error", tt.args, err)
			}

			if diff := cmp.Diff(tt.want, options); diff != "" {
				t.Errorf("options (-want +got):\n%s", diff)
			}
		})
	}
}

func TestACreateProjectInstructionItCannotReadIsAUsageRefusal(t *testing.T) {
	cases := map[string]struct {
		args []string
		kind string
	}{
		"an unknown option":                 {[]string{"-x"}, "invalid_argument"},
		"an unknown long option":            {[]string{"--domain=shop.test"}, "invalid_argument"},
		"a stray argument":                  {[]string{"-p", "shop", "extra"}, "unexpected_argument"},
		"an edition there is no project of": {[]string{"-e", "open-source"}, "invalid_edition"},
		"an edition in another case":        {[]string{"--edition=Community"}, "invalid_edition"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseCreateProject(tt.args, "hm")

			var refusal Refusal
			if !errors.As(err, &refusal) || refusal.Kind != tt.kind || refusal.Code != 2 {
				t.Fatalf("ParseCreateProject(%v) = %v, want a %s refusal with code 2", tt.args, err, tt.kind)
			}
		})
	}
}

// TestTheLatestVersionIsByReleaseOrder: as text 2.4.10 sorts before 2.4.9, and a patch release
// -p10 before -p9.
func TestTheLatestVersionIsByReleaseOrder(t *testing.T) {
	cases := map[string]struct {
		versions []string
		want     string
	}{
		"a two-digit release":       {[]string{"2.4.9", "2.4.10", "2.4.8-p5"}, "2.4.10"},
		"a patch after its release": {[]string{"2.4.7-p1", "2.4.7"}, "2.4.7-p1"},
		"a two-digit patch":         {[]string{"2.4.7-p9", "2.4.7-p10", "2.4.7-p2"}, "2.4.7-p10"},
		"nothing":                   {nil, ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			if got := LatestVersion(tt.versions); got != tt.want {
				t.Errorf("LatestVersion(%v) = %q, want %q", tt.versions, got, tt.want)
			}
		})
	}
}
