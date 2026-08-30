package app

import (
	"fmt"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core/ports"
)

// Doctor is the diagnosis: everything that has to be true for an environment to work, asked at
// once, each answer with the sentence that fixes it.
//
// What makes it worth running rather than reading is that most of these questions have an answer
// nobody thinks to look for. The most frequent reason an environment refuses to start is another
// project already listening on 80, and the second is a virtual machine somebody sized once and
// forgot — neither of which says so when it happens.
type Doctor struct {
	Daemon  ports.Daemon
	Engine  ports.ContainerEngine
	Compose ports.ComposeConfig
	Magento ports.MagentoFiles
	Tooling ports.Tooling
	State   ports.DataState
	Machine ports.Machine
	FS      ports.FS

	// Project is the one this directory belongs to, when it belongs to one. The checks that need
	// it declare so, and are skipped rather than failed when there is none: `hm doctor` in a home
	// directory is a legitimate thing to run, and it should answer about the machine.
	Project   core.Project
	InProject bool

	// ComposeFiles is the pair the project is built from, and Template is the one the tool ships
	// — used to know which ports an environment would need when there is no project to ask.
	ComposeFiles []string
	Template     string

	// Platform is "mac" or "linux": the conditions that matter are not the same on both.
	Platform string

	// Binary is the name the tool was invoked as, because every action printed is a command the
	// reader is meant to be able to paste.
	Binary string

	// Profile and Agent are what turn anonymisation from tidiness into compliance: an environment
	// an agent works in holding real customer data is a different fact from a developer's copy.
	Profile string
	Agent   string

	// Timeout bounds each check on its own. One that hangs must not take the diagnosis with it —
	// which is the whole reason the shell implementation ran each in its own process.
	Timeout time.Duration
}

// facts are what several checks need and none of them should ask for twice.
//
// The container list and the Compose configuration are the two expensive answers here, and the
// shell implementation paid for the configuration five separate times — once per check that
// wanted it. Loading them once is most of what this command gained by being ported.
type facts struct {
	containers []core.Container
	compose    core.Compose
	composeErr error
}

type check struct {
	id string

	// project marks a check that has nothing to say outside a configured project. Declaring it
	// here is what makes the scope a single fact rather than one repeated in every check.
	project bool

	ask func(Doctor, facts) []core.Finding
}

// checks, in the order they are reported. The order is the shell implementation's, which numbered
// the files: the machine first, then the project, then the things only a diagnosis would think to
// look at.
func (d Doctor) checks() []check {
	return []check{
		{id: "docker-daemon", ask: Doctor.dockerDaemon},
		{id: "compose-version", ask: Doctor.composeVersion},
		{id: "ports", ask: Doctor.ports},
		{id: "disk-usage", ask: Doctor.diskUsage},
		{id: "vm-resources", ask: Doctor.vmResources},
		{id: "certificates", ask: Doctor.certificates},
		{id: "platform", ask: Doctor.platform},
		{id: "compose-config", project: true, ask: Doctor.composeConfig},
		{id: "properties", project: true, ask: Doctor.properties},
		{id: "services", project: true, ask: Doctor.services},
		{id: "certificate", project: true, ask: Doctor.certificate},
		{id: "hosts", project: true, ask: Doctor.hosts},
		{id: "domain", project: true, ask: Doctor.domain},
		{id: "magento", project: true, ask: Doctor.magento},
		{id: "agent-context", project: true, ask: Doctor.agentContext},
		{id: "anonymised", project: true, ask: Doctor.anonymised},
		{id: "mail-image", project: true, ask: Doctor.mailImage},
	}
}

// Diagnose runs every check and reports what they found.
//
// Only is the id of a single check, for when somebody is working on one thing and does not want
// the other sixteen.
func (d Doctor) Diagnose(only string) core.Diagnosis {
	shared := d.gather()

	timeout := d.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	selected := []check{}
	for _, one := range d.checks() {
		if only != "" && one.id != only {
			continue
		}

		selected = append(selected, one)
	}

	//
	// The checks are independent and mostly spend their time waiting on something, so they run at
	// once: the diagnosis costs the slowest of them instead of the sum of all seventeen.
	//
	// Each writes into its own slot, so the report is in the declared order no matter who
	// finishes first — a diagnosis whose lines moved between runs would be read as a diagnosis
	// that changed.
	//
	answers := make([]chan []core.Finding, len(selected))

	for at, one := range selected {
		answers[at] = make(chan []core.Finding, 1)

		go func(one check, answer chan<- []core.Finding) {
			defer func() {
				// A check that panics is a bug in one check, not a reason to have no diagnosis
				if problem := recover(); problem != nil {
					answer <- []core.Finding{{
						Severity: core.SeverityWarning,
						Message:  fmt.Sprintf("Check failed unexpectedly (%v)", problem),
						Action:   d.Binary + " doctor --only=" + one.id,
					}}
				}
			}()

			if one.project && !d.InProject {
				answer <- nil

				return
			}

			answer <- one.ask(d, shared)
		}(one, answers[at])
	}

	diagnosis := core.Diagnosis{Checks: []core.Finding{}}

	for at, one := range selected {
		scope := "global"
		if one.project {
			scope = "project"
		}

		var found []core.Finding

		select {
		case found = <-answers[at]:
		case <-time.After(timeout):
			// The check keeps running and nobody waits for it. Saying so is the point: a silent
			// omission would read as "nothing to report about that"
			found = []core.Finding{{
				Scope:    "global",
				Severity: core.SeverityWarning,
				Message:  fmt.Sprintf("Check timed out after %ds", int(timeout.Seconds())),
				Action:   d.Binary + " doctor --only=" + one.id,
			}}
		}

		for _, finding := range found {
			finding.ID = one.id

			if finding.Scope == "" {
				finding.Scope = scope
			}

			diagnosis.Checks = append(diagnosis.Checks, finding)
		}
	}

	diagnosis.Summarise()

	return diagnosis
}

// gather asks the two expensive questions once, before anything else starts.
func (d Doctor) gather() facts {
	shared := facts{}
	done := make(chan struct{}, 2)

	go func() {
		shared.containers, _ = d.Engine.Containers()

		done <- struct{}{}
	}()

	go func() {
		if d.InProject {
			shared.compose, shared.composeErr = d.Compose.Load(d.Project.Root, d.Project.Name, d.ComposeFiles)
		}

		done <- struct{}{}
	}()

	<-done
	<-done

	return shared
}

func ok(message string, action ...string) []core.Finding {
	return one(core.SeverityOK, message, action...)
}

func warn(message string, action ...string) []core.Finding {
	return one(core.SeverityWarning, message, action...)
}

func fail(message string, action ...string) []core.Finding {
	return one(core.SeverityError, message, action...)
}

func one(severity, message string, action ...string) []core.Finding {
	finding := core.Finding{Severity: severity, Message: message}
	if len(action) > 0 {
		finding.Action = action[0]
	}

	return []core.Finding{finding}
}
