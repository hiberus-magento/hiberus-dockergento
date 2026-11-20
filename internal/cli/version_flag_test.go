package cli

import (
	"bytes"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// `hm --version`, as process_version in console/helpers/process_hm_options.sh printed it: the
// installation and nothing underneath it, which is what keeps it the startup floor. The `version`
// subcommand is the long answer and is tested in version_test.go.
//

func versionFlagWith(t *testing.T, installed core.Installation) *fakeEngine {
	t.Helper()

	fake := answering(t)
	pinHelp(t, "/opt/hm", "mac")

	original := installation
	installation = func(root string) core.Installation {
		installed.Path = root

		return installed
	}
	t.Cleanup(func() { installation = original })

	return fake
}

var aheadOfATag = core.Installation{
	Version: "1.6.0-rc.2-5-gabc1234", Tag: "1.6.0-rc.2", CommitsAhead: 5, Commit: "abc1234",
	Branch: "release/2.0.0",
}

func TestTheVersionFlagNamesTheInstallation(t *testing.T) {
	fake := versionFlagWith(t, aheadOfATag)
	t.Setenv("NO_COLOR", "1")

	for _, flag := range []string{"--version", "-v"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{flag, "--no-json"}, &stdout, &stderr)

		want := "hm 1.6.0-rc.2-5-gabc1234\n" +
			"  branch       release/2.0.0\n" +
			"  commit       abc1234\n" +
			"  installed    /opt/hm\n" +
			"\n"

		if code != 0 || stdout.String() != want || stderr.String() != "" {
			t.Fatalf("%s = %d %q %q, want %q", flag, code, stdout.String(), stderr.String(), want)
		}
	}

	// The startup floor asks Docker nothing
	if len(fake.calls) != 0 {
		t.Fatalf("--version asked the engine %v, want nothing", fake.calls)
	}
}

func TestADetachedDirtyInstallationSaysSo(t *testing.T) {
	versionFlagWith(t, core.Installation{Version: "unknown", Detached: true, Dirty: true})
	t.Setenv("FORCE_COLOR", "1")

	var stdout, stderr bytes.Buffer
	Run([]string{"--version", "--no-json"}, &stdout, &stderr)

	want := "\033[1mhm unknown\n\033[0m" +
		"  version      unknown (detached checkout)\n" +
		"  commit       unknown\n" +
		"  installed    /opt/hm\n" +
		"\033[0;33m  uncommitted changes in the installation directory\n\033[0m" +
		"\n"

	if stdout.String() != want {
		t.Fatalf("got  %q\nwant %q", stdout.String(), want)
	}
}

func TestTheVersionFlagUnderJSONIsTheShellsEnvelope(t *testing.T) {
	versionFlagWith(t, aheadOfATag)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--json", "--version"}, &stdout, &stderr)

	want := `{
  "schema_version": 1,
  "command": "version",
  "ok": true,
  "data": {
    "version": "1.6.0-rc.2-5-gabc1234",
    "tag": "1.6.0-rc.2",
    "commits_ahead": 5,
    "commit": "abc1234",
    "branch": "release/2.0.0",
    "detached": false,
    "dirty": false,
    "path": "/opt/hm"
  }
}
`
	if code != 0 || stdout.String() != want {
		t.Fatalf("= %d\n%s\nwant\n%s", code, stdout.String(), want)
	}
}

func TestOnlyTheFirstArgumentAsksForTheVersion(t *testing.T) {
	versionFlagWith(t, aheadOfATag)
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	Run([]string{"-v", "whatever", "--no-json"}, &stdout, &stderr)

	if !bytes.HasPrefix(stdout.Bytes(), []byte("hm 1.6.0-rc.2-5-gabc1234\n")) {
		t.Fatalf("-v whatever = %q, want the version", stdout.String())
	}
}
