package app

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The rules the shell's own tests pinned about individual checks and about the runner, asked of
// Doctor.Diagnose with the fakes doctor_test.go declares. Ported from tests/unit/vm_resources_test.sh
// and tests/integration/doctor_test.sh; what doctor_test.go already pins is not repeated here.
//

const gb = int64(1) << 30

// TestTheVMMemoryVerdict: on macOS the containers run in a VM with whatever memory somebody gave
// it once, and a machine with 48 GB whose VM has 6 fits about six environments while nothing says
// so. The thresholds are core's: under 4 GB nothing fits; on a machine of 16 GB or more, under a
// quarter of it is a setting somebody forgot.
func TestTheVMMemoryVerdict(t *testing.T) {
	const (
		colima  = "colima stop && colima start --memory 16 --cpu 4"
		desktop = "Docker Desktop → Settings → Resources → Memory"
	)

	cases := []struct {
		name    string
		vm      int64
		host    int64
		runtime string
		want    core.Finding
	}{
		{"a fair share of a large machine is fine", 16 * gb, 48 * gb, "colima", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 16 GB and 4 CPU: about 27 environments fit at once"}},
		{"and so is one on a machine with nothing to spare", 6 * gb, 8 * gb, "colima", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 6 GB and 4 CPU: about 9 environments fit at once"}},
		{"a sliver of a large machine on Colima says how to raise it", 6 * gb, 48 * gb, "colima", core.Finding{
			Severity: core.SeverityWarning, Message: "Docker has 6 GB of this machine's 48 GB, so about 9 environments fit at once",
			Action: colima}},
		{"and on Docker Desktop says where", 6 * gb, 48 * gb, "docker-desktop", core.Finding{
			Severity: core.SeverityWarning, Message: "Docker has 6 GB of this machine's 48 GB, so about 9 environments fit at once",
			Action: desktop}},
		{"and on a runtime it does not know says nothing about how", 6 * gb, 48 * gb, "orbstack", core.Finding{
			Severity: core.SeverityWarning, Message: "Docker has 6 GB of this machine's 48 GB, so about 9 environments fit at once"}},
		{"under four gigabytes nothing fits on a large machine", 3 * gb, 48 * gb, "colima", core.Finding{
			Severity: core.SeverityError, Message: "Docker has 3 GB and 4 CPU: not enough for one full stack", Action: colima}},
		{"nor on a small one", 3 * gb, 4 * gb, "docker-desktop", core.Finding{
			Severity: core.SeverityError, Message: "Docker has 3 GB and 4 CPU: not enough for one full stack", Action: desktop}},
		{"on Linux, where both numbers are the same, there is nothing to say", 32 * gb, 32 * gb, "", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 32 GB and 4 CPU: about 57 environments fit at once"}},
		{"a number nobody could read is not a verdict", 0, 48 * gb, "colima", core.Finding{
			Severity: core.SeverityWarning, Message: "Could not read how much memory Docker has"}},

		// The edges, so a threshold that moves shows up here
		{"exactly four gigabytes is enough", 4 * gb, 8 * gb, "", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 4 GB and 4 CPU: about 5 environments fit at once"}},
		{"a byte under four is not", 4*gb - 1, 8 * gb, "", core.Finding{
			Severity: core.SeverityError, Message: "Docker has 4 GB and 4 CPU: not enough for one full stack"}},
		{"exactly a quarter is a fair share", 12 * gb, 48 * gb, "", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 12 GB and 4 CPU: about 20 environments fit at once"}},
		{"a byte under a quarter is not", 12*gb - 1, 48 * gb, "", core.Finding{
			Severity: core.SeverityWarning, Message: "Docker has 12 GB of this machine's 48 GB, so about 20 environments fit at once"}},
		{"a machine of sixteen gigabytes has something to spare", 4 * gb, 16 * gb, "", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 4 GB and 4 CPU: about 5 environments fit at once"}},
		{"one of twenty is held to its quarter", 4 * gb, 20 * gb, "", core.Finding{
			Severity: core.SeverityWarning, Message: "Docker has 4 GB of this machine's 20 GB, so about 5 environments fit at once"}},
		{"one a byte under sixteen has nothing to spare", 4 * gb, 16*gb - 1, "", core.Finding{
			Severity: core.SeverityOK, Message: "Docker has 4 GB and 4 CPU: about 5 environments fit at once"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			physician := doctorFor(false)
			physician.Daemon = daemon{reachable: true, info: core.DaemonInfo{MemoryBytes: tt.vm, CPUs: 4, Runtime: tt.runtime}}
			physician.Machine = host{memory: tt.host, disk: 100}

			want := tt.want
			want.ID = "vm-resources"
			want.Scope = "global"

			if diff := cmp.Diff([]core.Finding{want}, findingsFor(physician.Diagnose("vm-resources"), "vm-resources")); diff != "" {
				t.Fatalf("vm-resources (-want +got):\n%s", diff)
			}
		})
	}
}

// TestADaemonThatCannotSayItsMemoryIsNotAVerdict: the same sentence as a zero, with no action.
func TestADaemonThatCannotSayItsMemoryIsNotAVerdict(t *testing.T) {
	physician := doctorFor(false)
	physician.Daemon = daemon{reachable: true, infoErr: errStub}

	want := []core.Finding{{ID: "vm-resources", Scope: "global", Severity: core.SeverityWarning,
		Message: "Could not read how much memory Docker has"}}

	if diff := cmp.Diff(want, findingsFor(physician.Diagnose("vm-resources"), "vm-resources")); diff != "" {
		t.Fatalf("vm-resources (-want +got):\n%s", diff)
	}
}

var errStub = stubError("docker info failed")

type stubError string

func (e stubError) Error() string { return string(e) }

type panickingDaemon struct{ daemon }

func (panickingDaemon) Info() (core.DaemonInfo, error) { panic("a bug in one check") }

// TestACheckThatPanicsIsAWarningAndTheRestStillRun: a check that panics is a bug in one check,
// not a reason to have no diagnosis — it is reported as a warning pointing at itself, and every
// other check is still reported.
func TestACheckThatPanicsIsAWarningAndTheRestStillRun(t *testing.T) {
	healthy := doctorFor(false).Diagnose("")

	physician := doctorFor(false)
	physician.Daemon = panickingDaemon{daemon{reachable: true, local: true, pullable: true}}

	diagnosis := physician.Diagnose("")

	want := []core.Finding{{ID: "vm-resources", Scope: "global", Severity: core.SeverityWarning,
		Message: "Check failed unexpectedly (a bug in one check)", Action: "hm doctor --only=vm-resources"}}

	if diff := cmp.Diff(want, findingsFor(diagnosis, "vm-resources")); diff != "" {
		t.Fatalf("the check that panicked (-want +got):\n%s", diff)
	}

	if len(diagnosis.Checks) != len(healthy.Checks) {
		t.Fatalf("checks with one panicking = %d, want the %d of a healthy diagnosis", len(diagnosis.Checks), len(healthy.Checks))
	}

	for at, finding := range diagnosis.Checks {
		if finding.ID == "vm-resources" {
			continue
		}

		if diff := cmp.Diff(healthy.Checks[at], finding); diff != "" {
			t.Errorf("check %q changed because another panicked (-healthy +got):\n%s", finding.ID, diff)
		}
	}
}

// TestWarningsAloneDoNotMakeTheSummaryAnError: the summary counts each severity on its own, and a
// diagnosis of nothing but warnings has no errors — which is what keeps `hm doctor` at exit 0.
func TestWarningsAloneDoNotMakeTheSummaryAnError(t *testing.T) {
	// Outside a project, with no mkcert and no template to read ports from: two warnings, the
	// rest OK
	diagnosis := doctorFor(false).Diagnose("")

	warnings, oks := 0, 0
	for _, finding := range diagnosis.Checks {
		switch finding.Severity {
		case core.SeverityWarning:
			warnings++
		case core.SeverityOK:
			oks++
		default:
			t.Fatalf("check %q is %q, want this fixture to produce no errors", finding.ID, finding.Severity)
		}
	}

	if warnings == 0 {
		t.Fatalf("warnings = 0, want this fixture to produce some")
	}

	summary := diagnosis.Summary
	if summary.Errors != 0 || summary.Warnings != warnings || summary.OK != oks || summary.Total != len(diagnosis.Checks) {
		t.Fatalf("summary = %+v, want 0 errors, %d warnings, %d ok, %d total",
			summary, warnings, oks, len(diagnosis.Checks))
	}
}

// TestInsideAProjectTheProjectChecksRunToo: every check declared for a project reports, under the
// project scope, alongside the machine's.
func TestInsideAProjectTheProjectChecksRunToo(t *testing.T) {
	diagnosis := doctorFor(true).Diagnose("")

	for _, id := range []string{
		"compose-config", "properties", "services", "certificate", "hosts",
		"domain", "magento", "agent-context", "anonymised", "mail-image",
	} {
		found := findingsFor(diagnosis, id)
		if len(found) == 0 {
			t.Errorf("project check %q did not report inside a project", id)

			continue
		}

		for _, finding := range found {
			if finding.Scope != "project" {
				t.Errorf("check %q scope = %q, want project", id, finding.Scope)
			}
		}
	}

	if len(findingsFor(diagnosis, "docker-daemon")) != 1 {
		t.Errorf("machine checks inside a project = none, want them to run as well")
	}
}

// TestTheMagentoCheck: a project with Magento reports its version, and one without Magento is
// reported without failing the diagnosis.
func TestTheMagentoCheck(t *testing.T) {
	cases := []struct {
		name    string
		present []string
		version string
		want    core.Finding
	}{
		{"an installed Magento reports its version", []string{"/code/shop/composer.lock", "/code/shop/app/etc/env.php"}, "2.4.9",
			core.Finding{Severity: core.SeverityOK, Message: "Magento 2.4.9"}},
		{"one not installed yet still reports the real version", []string{"/code/shop/composer.lock"}, "2.4.9",
			core.Finding{Severity: core.SeverityWarning, Message: "Magento 2.4.9 is not installed yet (no app/etc/env.php)",
				Action: "hm install"}},
		{"a composer.lock with no Magento package", []string{"/code/shop/composer.lock"}, "",
			core.Finding{Severity: core.SeverityWarning, Message: "composer.lock has no Magento package",
				Action: "Check that this is a Magento project"}},
		{"no composer.lock at all", nil, "",
			core.Finding{Severity: core.SeverityWarning, Message: "No composer.lock found in .",
				Action: "hm composer install"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			present := map[string]bool{}
			for _, path := range tt.present {
				present[path] = true
			}

			physician := doctorFor(true)
			physician.FS = filesystem{present: present}
			physician.Magento = magento{version: tt.version, admin: "admin"}

			diagnosis := physician.Diagnose("magento")

			want := tt.want
			want.ID = "magento"
			want.Scope = "project"

			if diff := cmp.Diff([]core.Finding{want}, diagnosis.Checks); diff != "" {
				t.Fatalf("magento (-want +got):\n%s", diff)
			}

			if strings.Contains(diagnosis.Checks[0].Message, "has no Magento package") && tt.version != "" {
				t.Errorf("a project with Magento %s reported as lacking it", tt.version)
			}

			if diagnosis.Summary.Errors != 0 {
				t.Errorf("summary errors = %d, want the Magento check never to fail the diagnosis", diagnosis.Summary.Errors)
			}
		})
	}
}

// TestAMissingHostEntryIsAWarningWithAnAction: the domain without a line in /etc/hosts is said,
// with the command that adds it; one with its line resolves.
func TestAMissingHostEntryIsAWarningWithAnAction(t *testing.T) {
	cases := []struct {
		name  string
		entry bool
		want  core.Finding
	}{
		{"no entry", false, core.Finding{Severity: core.SeverityWarning, Message: "shop.test has no entry in /etc/hosts",
			Action: "hm set-host shop.test --no-database"}},
		{"its entry", true, core.Finding{Severity: core.SeverityOK, Message: "shop.test resolves locally"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			physician := doctorFor(true)
			physician.Machine = host{memory: 16 << 30, disk: 100, hosts: tt.entry}

			want := tt.want
			want.ID = "hosts"
			want.Scope = "project"

			if diff := cmp.Diff([]core.Finding{want}, physician.Diagnose("hosts").Checks); diff != "" {
				t.Fatalf("hosts (-want +got):\n%s", diff)
			}
		})
	}
}
