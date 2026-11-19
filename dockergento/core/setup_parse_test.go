package core

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What setup's parser takes. Ported from tests/unit/setup_options_test.sh, which tested the same
// rules on the shell implementation's parser; setup_test.go already pins Warden's spellings and an
// undeclared option.
//

func TestEveryDocumentedSetupOptionIsTaken(t *testing.T) {
	want := SetupOptions{
		Dump: "/x.sql", Install: true, ProjectName: "shop", Domain: "shop.test", Root: "src",
		Force: true, UseDefault: true, Mail: "mailpit",
	}

	cases := map[string][]string{
		"long forms with an equals": {"--dump=/x.sql", "--install", "--project-name=shop",
			"--domain=shop.test", "--root-directory=src", "--force", "--use-default", "--mail=mailpit"},
		"long forms with a space": {"--dump", "/x.sql", "--install", "--project-name", "shop",
			"--domain", "shop.test", "--root-directory", "src", "--force", "--use-default", "--mail", "mailpit"},
		"short forms": {"-D", "/x.sql", "-i", "-p", "shop", "-d", "shop.test", "-r", "src", "-f", "-u",
			"--mail", "mailpit"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			options, err := ParseSetup(args, "hm")
			if err != nil {
				t.Fatalf("ParseSetup(%v) = %v, want no error", args, err)
			}

			if diff := cmp.Diff(want, options); diff != "" {
				t.Errorf("options (-want +got):\n%s", diff)
			}
		})
	}
}

// TestNothingGivenMeansNothingDecided: no options is the zero instruction, for the questions to fill.
func TestNothingGivenMeansNothingDecided(t *testing.T) {
	options, err := ParseSetup(nil, "hm")
	if err != nil || options != (SetupOptions{}) {
		t.Fatalf("ParseSetup(nil) = %+v, %v, want nothing decided and no error", options, err)
	}
}

// TestAnArgumentThatIsNotAnOptionIsRefused: setup takes no positional arguments, and a stray word
// is a usage error rather than something silently ignored.
func TestAnArgumentThatIsNotAnOptionIsRefused(t *testing.T) {
	_, err := ParseSetup([]string{"shop"}, "hm")

	var refusal Refusal
	if !errors.As(err, &refusal) || refusal.Code != 2 || refusal.Kind != "unexpected_argument" {
		t.Fatalf("ParseSetup([shop]) = %v, want an unexpected_argument refusal with code 2", err)
	}
}
