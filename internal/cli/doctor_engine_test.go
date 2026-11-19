package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What doctor asks of the engine, and how it reports the answer. doctor_test.go pins what it
// refuses while parsing; this file pins what it asks once the arguments are good and how a
// diagnosis becomes an exit code, a JSON envelope or text. What the checks find is app.Doctor's,
// pinned in dockergento/app.
//

// diagnosed is a diagnosis with its summary counted the way app.Doctor counts it.
func diagnosed(findings ...core.Finding) core.Diagnosis {
	diagnosis := core.Diagnosis{Checks: findings}
	diagnosis.Summarise()

	return diagnosis
}

func TestWhatDoctorAsks(t *testing.T) {
	cases := []struct {
		name string
		args []string
		only string
	}{
		{"every check", nil, ""},
		{"--only names one", []string{"--only=ports"}, "ports"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			cwd := here()

			if code := doctor(tt.args, io.Discard, io.Discard, false); code != exitOK {
				t.Fatalf("doctor %v = %d, want exitOK", tt.args, code)
			}

			want := []call{{Method: "Diagnose", Dir: cwd, Only: tt.only}}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestTheDiagnosisDecidesTheExitCode: a script or a CI job must not have to read the report to
// know whether something is broken, and a warning is not broken.
func TestTheDiagnosisDecidesTheExitCode(t *testing.T) {
	cases := []struct {
		name      string
		diagnosis core.Diagnosis
		want      int
	}{
		{"warnings alone keep it at zero", diagnosed(
			core.Finding{ID: "docker-daemon", Severity: core.SeverityOK, Message: "Docker daemon is running"},
			core.Finding{ID: "disk-usage", Severity: core.SeverityWarning, Message: "120 Docker volumes"},
		), exitOK},
		{"one error fails it", diagnosed(
			core.Finding{ID: "disk-usage", Severity: core.SeverityWarning, Message: "120 Docker volumes"},
			core.Finding{ID: "ports", Severity: core.SeverityError, Message: "Ports 80 are taken"},
		), exitError},
	}

	for _, tt := range cases {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(tt.name, func(t *testing.T) {
				fake := answering(t)
				fake.outcomes = []outcome{{diagnosis: tt.diagnosis}}

				var stderr bytes.Buffer
				if code := doctor(nil, io.Discard, &stderr, jsonOutput); code != tt.want {
					t.Fatalf("doctor (json=%v) = %d, want %d", jsonOutput, code, tt.want)
				}

				// The verdict is the report's, not a failure of the command: nothing on stderr
				if stderr.Len() != 0 {
					t.Errorf("stderr = %q, want nothing", stderr.String())
				}
			})
		}
	}
}

// TestDoctorReportsTheDiagnosisAsAnEnvelope: under --json, stdout is the contract's envelope and
// its data is the diagnosis itself — every check with its five fields, and the summary.
func TestDoctorReportsTheDiagnosisAsAnEnvelope(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{diagnosis: diagnosed(
		core.Finding{ID: "docker-daemon", Scope: "global", Severity: core.SeverityOK, Message: "Docker daemon is running"},
		core.Finding{ID: "hosts", Scope: "project", Severity: core.SeverityWarning,
			Message: "shop.test has no entry in /etc/hosts", Action: "hm set-host shop.test --no-database"},
	)}}

	var stdout bytes.Buffer
	if code := doctor(nil, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("doctor --json = %d, want exitOK", code)
	}

	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}

	want := map[string]any{
		"schema_version": envelope["schema_version"],
		"command":        "doctor",
		"ok":             true,
		"data": map[string]any{
			"checks": []any{
				map[string]any{"id": "docker-daemon", "scope": "global", "severity": "ok",
					"message": "Docker daemon is running", "action": ""},
				map[string]any{"id": "hosts", "scope": "project", "severity": "warning",
					"message": "shop.test has no entry in /etc/hosts", "action": "hm set-host shop.test --no-database"},
			},
			"summary": map[string]any{"total": 2.0, "ok": 1.0, "warnings": 1.0, "errors": 0.0},
		},
	}

	if diff := cmp.Diff(want, envelope); diff != "" {
		t.Fatalf("envelope (-want +got):\n%s", diff)
	}

	if _, versioned := envelope["schema_version"].(float64); !versioned {
		t.Errorf("schema_version = %v, want a number", envelope["schema_version"])
	}
}

// TestDoctorReportsTheDiagnosisAsText: each check's ID and message, the action under a check
// that is not OK — and never under one that is, where it would read as something to do.
func TestDoctorReportsTheDiagnosisAsText(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{diagnosis: diagnosed(
		core.Finding{ID: "docker-daemon", Severity: core.SeverityOK, Message: "Docker daemon is running",
			Action: "an action nobody should see"},
		core.Finding{ID: "hosts", Severity: core.SeverityWarning,
			Message: "shop.test has no entry in /etc/hosts", Action: "hm set-host shop.test --no-database"},
		core.Finding{ID: "ports", Severity: core.SeverityError,
			Message: "Ports 80 are taken by the 'otra' environment", Action: "cd into that project and run 'hm stop'"},
	)}}

	var stdout bytes.Buffer
	if code := doctor(nil, &stdout, io.Discard, false); code != exitError {
		t.Fatalf("doctor = %d, want exitError", code)
	}

	text := stdout.String()

	for _, want := range []string{
		"docker-daemon", "Docker daemon is running",
		"hosts", "shop.test has no entry in /etc/hosts", "hm set-host shop.test --no-database",
		"ports", "Ports 80 are taken by the 'otra' environment", "cd into that project and run 'hm stop'",
		"1 error(s), 1 warning(s)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text report lacks %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, "an action nobody should see") {
		t.Errorf("text report prints the action of an OK check:\n%s", text)
	}
}

// TestADiagnosisThatCannotRunIsReported: an engine error is diagnosis_failed, with exitError,
// and nothing on stdout that a script could mistake for a report.
func TestADiagnosisThatCannotRunIsReported(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: errors.New("no state directory")}}

	var stdout, stderr bytes.Buffer
	if code := doctor(nil, &stdout, &stderr, true); code != exitError {
		t.Fatalf("doctor with the engine failing = %d, want exitError", code)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}

	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.OK || envelope.Error.Code != exitError || envelope.Error.Type != "diagnosis_failed" ||
		envelope.Error.Message != "no state directory" {
		t.Fatalf("error envelope = %+v, want diagnosis_failed with the engine's message", envelope)
	}
}
