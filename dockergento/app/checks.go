package app

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Is the Docker daemon reachable? Every other answer is meaningless when it is not.
func (d Doctor) dockerDaemon(facts) []core.Finding {
	if d.Daemon.Reachable() {
		return ok("Docker daemon is running")
	}

	return fail("Docker daemon is not running", "Start Docker and run the diagnosis again")
}

// Docker Compose present and recent enough.
func (d Doctor) composeVersion(facts) []core.Finding {
	version := d.Tooling.ComposeVersion()

	if version == "" {
		return fail("Docker Compose was not found", "Install Docker Compose v2")
	}

	if core.VersionAtLeast(version, "2.0.0") {
		return ok("Docker Compose " + version)
	}

	return warn("Docker Compose "+version+" is old and no longer tested", "Upgrade to Docker Compose v2")
}

// The ports the stack needs, and who is holding them.
//
// The most frequent reason an environment refuses to start is another project already listening
// on 80, 443 or 3306, and until this check existed nothing said so.
func (d Doctor) ports(shared facts) []core.Finding {
	wanted := d.wantedPorts(shared)
	if len(wanted) == 0 {
		return warn("Could not work out which ports this environment needs")
	}

	listeners, err := d.Machine.Listening()
	if err != nil {
		return warn("No tool available to inspect listening ports", "Install lsof to enable this check")
	}

	held := map[string]string{}
	for _, listener := range listeners {
		if _, seen := held[listener.Port]; !seen {
			held[listener.Port] = listener.Process
		}
	}

	// Who published what, from the containers rather than from the host: it is what turns "port
	// 80 is busy" into the name of the environment to go and stop
	owners := map[string]string{}
	for _, container := range shared.containers {
		if !container.Running {
			continue
		}

		for _, port := range container.Published {
			if _, seen := owners[port]; !seen {
				owners[port] = container.ComposeProject
			}
		}
	}

	taken := map[string][]string{}
	byOthers := []string{}
	hostPorts := []string{}

	for _, port := range wanted {
		if _, busy := held[port]; !busy {
			continue
		}

		owner := owners[port]

		if owner != "" && owner == d.Project.Name {
			continue
		}

		if owner != "" {
			taken[owner] = append(taken[owner], port)

			continue
		}

		hostPorts = append(hostPorts, port)
	}

	findings := []core.Finding{}
	problems := 0

	// Grouped by owner, and the owners sorted: one line naming the culprit and every port it
	// holds beats one line per port saying the same thing seven times
	for _, owner := range sorted(taken) {
		if !d.InProject {
			byOthers = append(byOthers, owner)

			continue
		}

		problems++
		findings = append(findings, core.Finding{
			Severity: core.SeverityError,
			Message:  fmt.Sprintf("Ports %s are taken by the '%s' environment", strings.Join(taken[owner], ", "), owner),
			Action:   fmt.Sprintf("cd into that project and run '%s stop'", d.Binary),
		})
	}

	if len(hostPorts) > 0 {
		process := held[hostPorts[0]]
		if process == "" {
			process = "processes on the host"
		}

		message := fmt.Sprintf("Ports %s are taken by %s", strings.Join(hostPorts, " "), process)

		if d.InProject {
			problems++
			findings = append(findings, core.Finding{
				Severity: core.SeverityError,
				Message:  message,
				Action:   "Stop whatever is listening on those ports",
			})
		} else {
			findings = append(findings, core.Finding{
				Severity: core.SeverityWarning,
				Message:  message,
				Action:   "They will clash with any environment that needs them",
			})
		}
	}

	if len(byOthers) > 0 {
		findings = append(findings, core.Finding{
			Severity: core.SeverityOK,
			Message:  "Ports held by running environments: " + strings.Join(byOthers, " "),
		})
	}

	if problems == 0 && len(byOthers) == 0 && len(hostPorts) == 0 {
		findings = append(findings, core.Finding{
			Severity: core.SeverityOK,
			Message:  "Every required port is free",
		})
	}

	return findings
}

// wantedPorts is what this environment publishes, or what one would publish here.
//
// The list comes from the configuration and is never duplicated in the check. Outside a project
// there is no configuration to ask, so the template the tool ships answers instead — which is the
// right answer to "would an environment start here".
func (d Doctor) wantedPorts(shared facts) []string {
	if d.InProject {
		seen := map[string]bool{}
		ports := []string{}

		for _, service := range shared.compose.Services {
			for _, port := range service.Ports {
				if port.Published == "" || seen[port.Published] {
					continue
				}

				seen[port.Published] = true
				ports = append(ports, port.Published)
			}
		}

		sort.Strings(ports)

		return ports
	}

	if d.Template == "" {
		return nil
	}

	published := regexp.MustCompile(`(?m)^ *- (\d+):\d+`)
	seen := map[int]bool{}
	numbers := []int{}

	for _, match := range published.FindAllStringSubmatch(d.FS.Read(d.Template), -1) {
		port, err := strconv.Atoi(match[1])
		if err != nil || seen[port] {
			continue
		}

		seen[port] = true
		numbers = append(numbers, port)
	}

	sort.Ints(numbers)

	ports := make([]string, 0, len(numbers))
	for _, port := range numbers {
		ports = append(ports, strconv.Itoa(port))
	}

	return ports
}

// Docker leftovers.
//
// `docker system df` computes real sizes and took 18s on a developer machine with 152 volumes,
// which is unusable in a diagnosis. Counting them costs a fraction of that and catches the same
// problem: an environment graveyard nobody cleans up.
func (d Doctor) diskUsage(facts) []core.Finding {
	volumes, dangling, err := d.Daemon.Leftovers()
	if err != nil {
		return warn("Could not read Docker volumes")
	}

	if volumes >= 100 {
		return warn(
			fmt.Sprintf("%d Docker volumes and %d dangling images on this machine", volumes, dangling),
			fmt.Sprintf("%s list  # look for orphaned environments", d.Binary))
	}

	return ok(fmt.Sprintf("%d Docker volumes, %d dangling images", volumes, dangling))
}

// Do the containers have enough memory, and enough of the machine's?
func (d Doctor) vmResources(facts) []core.Finding {
	info, err := d.Daemon.Info()
	if err != nil {
		return warn("Could not read how much memory Docker has")
	}

	host := d.Machine.MemoryBytes()
	vmGB := core.Gigabytes(info.MemoryBytes)
	hostGB := core.Gigabytes(host)
	environments := core.EnvironmentsThatFit(info.MemoryBytes)

	action := ""
	switch info.Runtime {
	case "colima":
		action = "colima stop && colima start --memory 16 --cpu 4"
	case "docker-desktop":
		action = "Docker Desktop → Settings → Resources → Memory"
	}

	switch core.VMMemoryVerdict(info.MemoryBytes, host) {
	case "small":
		return fail(fmt.Sprintf("Docker has %d GB and %d CPU: not enough for one full stack", vmGB, info.CPUs), action)
	case "cramped":
		return warn(fmt.Sprintf("Docker has %d GB of this machine's %d GB, so about %d environments fit at once",
			vmGB, hostGB, environments), action)
	case "unknown":
		return warn("Could not read how much memory Docker has")
	default:
		return ok(fmt.Sprintf("Docker has %d GB and %d CPU: about %d environments fit at once",
			vmGB, info.CPUs, environments))
	}
}

// The local certificate authority.
func (d Doctor) certificates(facts) []core.Finding {
	installed, caroot := d.Machine.Mkcert()

	if !installed {
		return warn("mkcert is not installed, so HTTPS certificates cannot be issued", d.Binary+" ssl")
	}

	if caroot == "" || !d.FS.Exists(filepath.Join(caroot, "rootCA.pem")) {
		return warn("mkcert is installed but its local authority is missing", "mkcert -install")
	}

	return ok("mkcert is installed and its local authority is in place")
}

// The conditions that are not the same on both platforms.
func (d Doctor) platform(facts) []core.Finding {
	if d.Platform == "linux" {
		if d.Machine.InGroup("docker") {
			return ok("User belongs to the docker group")
		}

		return fail("User does not belong to the docker group",
			"sudo usermod -aG docker $USER  # then log out and back in")
	}

	available, readable := d.Machine.FreeDiskGB()

	if readable && available < 10 {
		return warn(fmt.Sprintf("Only %dGB free on the startup disk",
			available), "Free up disk space: Docker Desktop fails in confusing ways when it runs out")
	}

	if !readable {
		return ok("Disk space available: unknownGB")
	}

	return ok(fmt.Sprintf("Disk space available: %dGB", available))
}

// Is the Compose configuration of this project valid?
func (d Doctor) composeConfig(shared facts) []core.Finding {
	if shared.composeErr == nil {
		return ok("Docker Compose configuration is valid")
	}

	reason := shared.composeErr.Error()
	if reason == "" {
		reason = "unknown reason"
	}

	return fail("Docker Compose configuration is invalid: "+reason, d.Binary+" setup -f")
}

// The project's own properties.
func (d Doctor) properties(facts) []core.Finding {
	file := filepath.Join(d.Project.Root, "config", "docker", "properties.json")

	values := map[string]any{}
	if err := json.Unmarshal([]byte(d.FS.Read(file)), &values); err != nil {
		return fail(file+" is not valid JSON", d.Binary+" setup")
	}

	missing := []string{}
	for _, key := range []string{"COMPOSE_PROJECT_NAME", "DOMAIN", "MAGENTO_DIR"} {
		if text, _ := values[key].(string); text == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return warn("Project properties are missing: "+strings.Join(missing, " "), d.Binary+" setup")
	}

	if !d.FS.IsDir(filepath.Join(d.Project.Root, d.Project.MagentoDir)) {
		return fail(fmt.Sprintf("The Magento directory '%s' does not exist", d.Project.MagentoDir), d.Binary+" setup")
	}

	return ok("Project properties are complete")
}

// Are the services of this project up?
func (d Doctor) services(shared facts) []core.Finding {
	total := len(shared.compose.Services)
	running := 0

	for _, container := range shared.containers {
		if container.Key() == d.Project.Name && container.Running {
			running++
		}
	}

	switch {
	case total == 0:
		return warn("No services are defined for this project", d.Binary+" setup -f")
	case running == 0:
		return warn(fmt.Sprintf("The environment is stopped (%d services defined)", total), d.Binary+" start")
	case running < total:
		return warn(fmt.Sprintf("Only %d of %d services are running", running, total), d.Binary+" start")
	default:
		return ok(fmt.Sprintf("All %d services are running", total))
	}
}

// The certificate for this project's domain.
func (d Doctor) certificate(facts) []core.Finding {
	if d.Project.Domain == "" {
		return warn("This project has no domain configured", d.Binary+" set-host <domain>")
	}

	certificate := filepath.Join(d.Project.Root, "ssl.pem")

	if !d.FS.Exists(certificate) {
		return warn("No certificate found for "+d.Project.Domain, d.Binary+" ssl "+d.Project.Domain)
	}

	if expiresWithin(d.FS.Read(certificate), 7*24*time.Hour) {
		return warn("The certificate for "+d.Project.Domain+" expires within a week",
			d.Binary+" ssl "+d.Project.Domain)
	}

	return ok("Certificate for " + d.Project.Domain + " is valid")
}

// expiresWithin reads the certificate itself rather than asking openssl, which is one subprocess
// fewer and one machine fewer where the check silently did nothing because openssl was not there.
func expiresWithin(contents string, window time.Duration) bool {
	block, _ := pem.Decode([]byte(contents))
	if block == nil {
		return true
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true
	}

	return certificate.NotAfter.Before(time.Now().Add(window))
}

// Does the project domain resolve locally?
func (d Doctor) hosts(facts) []core.Finding {
	if d.Project.Domain == "" {
		return nil
	}

	if d.Machine.HostsEntry(d.Project.Domain) {
		return ok(d.Project.Domain + " resolves locally")
	}

	return warn(d.Project.Domain+" has no entry in /etc/hosts",
		d.Binary+" set-host "+d.Project.Domain+" --no-database")
}

// Does this project's domain reach this machine, and how?
//
// `hm set-host` asks for the system password to add a line to /etc/hosts, and the lines are never
// removed. None of that is needed when something already resolves the domain — so the check asks
// about the result, not about who produces it.
func (d Doctor) domain(facts) []core.Finding {
	domain := d.Project.Domain

	if domain == "" {
		return warn("This project has no domain configured", d.Binary+" setup --domain=project.test")
	}

	switch {
	case d.Machine.HostsEntry(domain):
		return ok(domain + " resolves here through /etc/hosts")
	case d.Machine.ResolvesLocally(domain):
		return ok(domain + " resolves here through DNS, with no /etc/hosts entry")
	default:
		return fail(domain+" does not resolve to this machine", d.Binary+" set-host "+domain)
	}
}

// The Magento installation.
func (d Doctor) magento(facts) []core.Finding {
	directory := d.Project.MagentoDir
	if directory == "" {
		directory = "."
	}

	if !d.FS.Exists(filepath.Join(d.Project.Root, directory, "composer.lock")) {
		return warn("No composer.lock found in "+directory, d.Binary+" composer install")
	}

	version := d.Magento.Version(d.Project.Root, directory)

	if version == "" {
		return warn("composer.lock has no Magento package", "Check that this is a Magento project")
	}

	if !d.FS.Exists(filepath.Join(d.Project.Root, directory, "app", "etc", "env.php")) {
		return warn("Magento "+version+" is not installed yet (no app/etc/env.php)", d.Binary+" install")
	}

	return ok("Magento " + version)
}

// The markers the generated block of AGENTS.md is written between.
const (
	contextBegin = "<!-- hm:begin - generated by hm ai-context, do not edit inside -->"
)

var recordedFingerprint = regexp.MustCompile(`hm:fingerprint ([a-f0-9]*)`)

// Does the generated agent context still describe this project?
//
// Not every project wants one, so its absence is a suggestion and not a failure. Its being wrong
// is another matter: an agent obeys what it reads, and a context that says PHP 7.4 in a project
// that moved to 8.2 is worse than no context at all.
func (d Doctor) agentContext(shared facts) []core.Finding {
	file := filepath.Join(d.Project.Root, "AGENTS.md")
	contents := d.FS.Read(file)

	if contents == "" || !strings.Contains(contents, contextBegin) {
		return ok("No generated agent context (optional: " + d.Binary + " ai-context)")
	}

	match := recordedFingerprint.FindStringSubmatch(contents)
	if match == nil || match[1] == "" {
		return warn("The agent context has no fingerprint and cannot be checked", d.Binary+" ai-context")
	}

	if match[1] == d.fingerprint(shared) {
		return ok("The agent context matches this project")
	}

	return fail("The agent context describes a different configuration than this project",
		d.Binary+" ai-context")
}

// fingerprint is what the context block was generated from.
//
// The hash is of the facts, not of the text, so re-running the generator on an unchanged project
// produces the same value and this check can compare. The shape is the shell implementation's,
// down to the sorted keys and the compact separators, because the value in AGENTS.md was written
// by it and has to keep matching.
func (d Doctor) fingerprint(shared facts) string {
	services := make([]map[string]string, 0, len(shared.compose.Services))
	for _, service := range shared.compose.Services {
		services = append(services, map[string]string{"image": service.Image, "name": service.Name})
	}

	anonymised, _ := d.State.Anonymisation(d.Project.Name)
	if anonymised == "" {
		anonymised = "unknown"
	}

	admin := ""
	if d.Project.Domain != "" {
		admin = "https://" + d.Project.Domain + "/" + d.Magento.AdminPath(d.Project.Root, d.Project.MagentoDir)
	}

	document, err := json.Marshal(map[string]any{
		"admin":      admin,
		"anonymised": anonymised,
		"domain":     d.Project.Domain,
		"magento":    d.Magento.Version(d.Project.Root, d.Project.MagentoDir),
		"services":   services,
	})
	if err != nil {
		return ""
	}

	// The newline jq leaves behind is part of what the shell implementation hashed, so it is part
	// of the value already written into people's AGENTS.md. Dropping it here would make every
	// generated context look stale, on every project, the day the binary shipped
	sum := md5.Sum(append(document, '\n'))

	return hex.EncodeToString(sum[:])
}

// Does an environment an agent works in hold data nobody anonymised?
//
// The question is only asked where it is compliance rather than tidiness: an environment on the
// agent profile, or one a tool has labelled as an agent's. Asking it of every project would
// produce a warning nobody reads on every project.
func (d Doctor) anonymised(facts) []core.Finding {
	state, at := d.State.Anonymisation(d.Project.Name)

	if state == "yes" {
		return ok("The database was anonymised on " + at)
	}

	if d.Profile == "agent" || d.Agent != "" {
		return fail("This environment is used by an agent and its database was never anonymised",
			d.Binary+" masquerade")
	}

	return ok("Database anonymisation: unknown (not required here)")
}

// Can the configured mail catcher's image be obtained?
//
// The Mailpit image is published to the registry by hand, outside the tool's release. Until
// somebody has pushed it, choosing Mailpit leaves the project pointing at something Docker cannot
// pull — and the natural failure for that is a half-created environment, at `up` time, in
// Docker's own wording. Saying it here turns it into a sentence that explains itself.
func (d Doctor) mailImage(shared facts) []core.Finding {
	image := ""

	for _, service := range shared.compose.Services {
		if service.Name == "mailpit" {
			image = service.Image

			break
		}

		if service.Name == "mailhog" {
			image = service.Image
		}
	}

	if image == "" {
		return warn("This project defines no mail catcher", d.Binary+" setup -f")
	}

	local, pullable := d.Daemon.ImageAvailability(image)

	switch {
	case local:
		return ok("The mail catcher image is available locally (" + image + ")")
	case pullable:
		return ok("The mail catcher image can be pulled (" + image + ")")
	default:
		return fail("The mail catcher image cannot be obtained: "+image,
			"That image is published manually; ask whoever maintains the registry, or set MAIL_SERVICE back to mailhog")
	}
}

func sorted(grouped map[string][]string) []string {
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
