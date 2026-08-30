package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

//
// The diagnosis is the command with the most outside world in it — a daemon, a host, a registry,
// a filesystem — and none of it is needed here. That is what the ports are for, and it is why
// the port conflict below can be exercised without holding port 80 on somebody's machine.
//

type daemon struct {
	reachable bool
	info      core.DaemonInfo
	infoErr   error
	volumes   int
	dangling  int
	local     bool
	pullable  bool
}

func (d daemon) Reachable() bool                { return d.reachable }
func (d daemon) Info() (core.DaemonInfo, error) { return d.info, d.infoErr }
func (d daemon) Leftovers() (int, int, error)   { return d.volumes, d.dangling, nil }
func (d daemon) ImageAvailability(string) (bool, bool) {
	return d.local, d.pullable
}

type host struct {
	memory    int64
	disk      int
	listening []core.Listener
	listenErr error
	group     bool
	caroot    string
	hosts     bool
	resolves  bool
}

func (h host) MemoryBytes() int64      { return h.memory }
func (h host) FreeDiskGB() (int, bool) { return h.disk, true }
func (h host) Listening() ([]core.Listener, error) {
	return h.listening, h.listenErr
}
func (h host) InGroup(string) bool         { return h.group }
func (h host) Mkcert() (bool, string)      { return h.caroot != "", h.caroot }
func (h host) HostsEntry(string) bool      { return h.hosts }
func (h host) ResolvesLocally(string) bool { return h.resolves }

type compose struct {
	configuration core.Compose
	err           error
}

func (c compose) Load(string, string, []string) (core.Compose, error) {
	return c.configuration, c.err
}

type magento struct{ version, mode, admin string }

func (m magento) Version(string, string) string   { return m.version }
func (m magento) Mode(string, string) string      { return m.mode }
func (m magento) AdminPath(string, string) string { return m.admin }

type tools struct{ compose string }

func (t tools) Version() string        { return "" }
func (t tools) ComposeVersion() string { return t.compose }
func (t tools) Workdir() string        { return "" }
func (t tools) Xdebug(string) string   { return "unknown" }

type state struct{ at string }

func (s state) Anonymisation(string) (string, string) {
	if s.at == "" {
		return "unknown", ""
	}

	return "yes", s.at
}

func doctorFor(project bool) Doctor {
	return Doctor{
		Daemon:  daemon{reachable: true, info: core.DaemonInfo{MemoryBytes: 16 << 30, CPUs: 4}, local: true, pullable: true},
		Engine:  engine{},
		Compose: compose{},
		Magento: magento{admin: "admin"},
		Tooling: tools{compose: "2.34.0"},
		State:   state{},
		Machine: host{memory: 16 << 30, disk: 100},
		FS:      filesystem{present: map[string]bool{}},

		Project:   core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test", MagentoDir: "."},
		InProject: project,
		Binary:    "hm",
	}
}

func findingsFor(diagnosis core.Diagnosis, id string) []core.Finding {
	found := []core.Finding{}

	for _, finding := range diagnosis.Checks {
		if finding.ID == id {
			found = append(found, finding)
		}
	}

	return found
}

func TestTheReportIsInTheSameOrderEveryTime(t *testing.T) {
	// Seventeen checks running at once, and a report whose lines moved between runs would be
	// read as a diagnosis that changed
	first := doctorFor(true).Diagnose("")
	second := doctorFor(true).Diagnose("")

	if len(first.Checks) != len(second.Checks) {
		t.Fatalf("dos diagnósticos del mismo proyecto con distinto número de líneas")
	}

	for at := range first.Checks {
		if first.Checks[at].ID != second.Checks[at].ID {
			t.Fatalf("en la posición %d salió %q y luego %q", at, first.Checks[at].ID, second.Checks[at].ID)
		}
	}

	if first.Checks[0].ID != "docker-daemon" {
		t.Fatalf("lo primero que hay que saber es si Docker responde, y salió %q", first.Checks[0].ID)
	}
}

func TestOutsideAProjectOnlyTheMachineIsDiagnosed(t *testing.T) {
	// `hm doctor` in a home directory is a legitimate thing to run, and it should answer about
	// the machine rather than complain about there being no project
	diagnosis := doctorFor(false).Diagnose("")

	for _, finding := range diagnosis.Checks {
		if finding.Scope == "project" {
			t.Fatalf("fuera de un proyecto no debería haber comprobaciones de proyecto: %q", finding.ID)
		}
	}

	if len(findingsFor(diagnosis, "docker-daemon")) != 1 {
		t.Fatalf("las comprobaciones de la máquina siguen corriendo fuera de un proyecto")
	}
}

func TestOnlyOneCheck(t *testing.T) {
	diagnosis := doctorFor(true).Diagnose("ports")

	if len(diagnosis.Checks) != 1 || diagnosis.Checks[0].ID != "ports" {
		t.Fatalf("--only debería dejar una sola comprobación, y dejó %d", len(diagnosis.Checks))
	}
}

func TestACheckThatHangsDoesNotTakeTheDiagnosisWithIt(t *testing.T) {
	physician := doctorFor(false)
	physician.Timeout = 20 * time.Millisecond
	physician.Daemon = slowDaemon{}

	diagnosis := physician.Diagnose("docker-daemon")

	if len(diagnosis.Checks) != 1 {
		t.Fatalf("la comprobación colgada debería reportarse igual, y salieron %d líneas", len(diagnosis.Checks))
	}

	if !strings.Contains(diagnosis.Checks[0].Message, "timed out") {
		t.Fatalf("una comprobación abandonada tiene que decirlo: %q", diagnosis.Checks[0].Message)
	}

	if diagnosis.Checks[0].Severity != core.SeverityWarning {
		t.Fatalf("no saber no es un fallo, es un aviso")
	}
}

type slowDaemon struct{ daemon }

func (slowDaemon) Reachable() bool {
	time.Sleep(2 * time.Second)

	return true
}

func TestAPortHeldByAnotherEnvironmentNamesIt(t *testing.T) {
	// "Port 80 is busy" is a fact; the name of the environment to go and stop is the sentence
	// somebody can act on
	physician := doctorFor(true)
	physician.Compose = compose{configuration: core.Compose{Services: []core.Service{
		{Name: "nginx", Ports: []core.Port{{Published: "80", Target: "80"}, {Published: "443", Target: "443"}}},
	}}}
	physician.Engine = engine{containers: []core.Container{
		{Running: true, ComposeProject: "otra-tienda", Published: []string{"80", "443"}},
	}}
	physician.Machine = host{
		memory:    16 << 30,
		listening: []core.Listener{{Port: "80", Process: "docker"}, {Port: "443", Process: "docker"}},
	}

	found := findingsFor(physician.Diagnose("ports"), "ports")

	if len(found) != 1 {
		t.Fatalf("dos puertos del mismo culpable son una línea, y salieron %d", len(found))
	}

	if found[0].Severity != core.SeverityError {
		t.Fatalf("un puerto ocupado impide arrancar: %q", found[0].Severity)
	}

	// Lexicographic, which is the order `jq unique` produced and therefore the order already in
	// everybody's output — not the order somebody would choose
	if !strings.Contains(found[0].Message, "443, 80") || !strings.Contains(found[0].Message, "otra-tienda") {
		t.Fatalf("el mensaje tiene que nombrar los puertos y el entorno: %q", found[0].Message)
	}
}

func TestOurOwnPortsAreNotAConflict(t *testing.T) {
	physician := doctorFor(true)
	physician.Compose = compose{configuration: core.Compose{Services: []core.Service{
		{Name: "nginx", Ports: []core.Port{{Published: "80", Target: "80"}}},
	}}}
	physician.Engine = engine{containers: []core.Container{
		{Running: true, ComposeProject: "shop", Published: []string{"80"}},
	}}
	physician.Machine = host{memory: 16 << 30, listening: []core.Listener{{Port: "80", Process: "docker"}}}

	found := findingsFor(physician.Diagnose("ports"), "ports")

	if len(found) != 1 || found[0].Severity != core.SeverityOK {
		t.Fatalf("el propio entorno escuchando en su puerto no es un conflicto: %+v", found)
	}
}

func TestAPortHeldByTheHostWithNoNameSaysSo(t *testing.T) {
	// `ss` names no process, and the shell implementation printed the word "LISTEN" as if it
	// were one
	physician := doctorFor(true)
	physician.Compose = compose{configuration: core.Compose{Services: []core.Service{
		{Name: "nginx", Ports: []core.Port{{Published: "80", Target: "80"}}},
	}}}
	physician.Machine = host{memory: 16 << 30, listening: []core.Listener{{Port: "80"}}}

	found := findingsFor(physician.Diagnose("ports"), "ports")

	if len(found) != 1 || !strings.Contains(found[0].Message, "processes on the host") {
		t.Fatalf("sin nombre de proceso el mensaje no puede inventarse uno: %+v", found)
	}
}

func TestWithNothingToLookWithNothingIsClaimed(t *testing.T) {
	physician := doctorFor(true)
	physician.Compose = compose{configuration: core.Compose{Services: []core.Service{
		{Name: "nginx", Ports: []core.Port{{Published: "80", Target: "80"}}},
	}}}
	physician.Machine = host{memory: 16 << 30, listenErr: errors.New("no hay herramienta")}

	found := findingsFor(physician.Diagnose("ports"), "ports")

	if len(found) != 1 || found[0].Severity != core.SeverityWarning {
		t.Fatalf("una comprobación que no puede mirar no puede decir que todo está libre: %+v", found)
	}
}

func TestAnAgentEnvironmentWithDataNobodyAnonymised(t *testing.T) {
	physician := doctorFor(true)
	physician.Profile = "agent"

	found := findingsFor(physician.Diagnose("anonymised"), "anonymised")

	if len(found) != 1 || found[0].Severity != core.SeverityError {
		t.Fatalf("en un entorno de agente esto es cumplimiento, no orden: %+v", found)
	}
}

func TestTheSameQuestionOfADeveloperCopyIsNotAFailure(t *testing.T) {
	found := findingsFor(doctorFor(true).Diagnose("anonymised"), "anonymised")

	if len(found) != 1 || found[0].Severity != core.SeverityOK {
		t.Fatalf("preguntárselo a todo el mundo sería un aviso que nadie lee: %+v", found)
	}
}

func TestTheContextFingerprintIsTheShellOnes(t *testing.T) {
	//
	// The value in AGENTS.md was written by the shell implementation, and this check compares
	// against it. The expected digest below is `jq -S -c` of the same facts piped through md5,
	// which is what generated the ones already sitting in people's checkouts.
	//
	physician := doctorFor(true)
	physician.Magento = magento{version: "2.4.7-p3", admin: "admin_9k2"}
	physician.Compose = compose{configuration: core.Compose{Services: []core.Service{
		{Name: "db", Image: "hiberusmagento/mariadb:10.6"},
		{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"},
	}}}

	shared := physician.gather()

	if fingerprint := physician.fingerprint(shared); fingerprint != "7a0329048d77eff6ce9873e3f8c8fef7" {
		t.Fatalf("la huella tiene que coincidir con la que escribió la versión bash, y salió %q", fingerprint)
	}
}
