package core

import "testing"

// The memory verdict is the one piece of the diagnosis with real judgement in it, and it is pure
// so that the judgement can be exercised without a daemon: the numbers below are the machines
// this was written for, not invented ones.

func TestAVirtualMachineTooSmallForOneStack(t *testing.T) {
	verdict := VMMemoryVerdict(2*gigabyte, 48*gigabyte)

	if verdict != "small" {
		t.Fatalf("con 2 GB no cabe un stack entero, y el veredicto fue %q", verdict)
	}
}

func TestALaptopWithPlentyAndAVirtualMachineWithout(t *testing.T) {
	// The case this check exists for: 48 GB of machine, 6 of Docker, and environments that will
	// not start on a laptop with memory to spare
	verdict := VMMemoryVerdict(6*gigabyte, 48*gigabyte)

	if verdict != "cramped" {
		t.Fatalf("6 GB de 48 es una VM estrecha, y el veredicto fue %q", verdict)
	}
}

func TestASmallMachineIsNotCramped(t *testing.T) {
	// 6 of 8 is the hardware, not a setting somebody forgot, and saying otherwise would be a
	// warning nobody can act on
	verdict := VMMemoryVerdict(6*gigabyte, 8*gigabyte)

	if verdict != "fine" {
		t.Fatalf("6 GB de 8 es lo que hay, y el veredicto fue %q", verdict)
	}
}

func TestNoAnswerIsNotABadAnswer(t *testing.T) {
	if verdict := VMMemoryVerdict(0, 48*gigabyte); verdict != "unknown" {
		t.Fatalf("sin dato no hay veredicto, y devolvió %q", verdict)
	}
}

func TestHowManyEnvironmentsFit(t *testing.T) {
	// Measured on the machine this was written on: eight environments in a 5.9 GiB virtual
	// machine, at about 550 MB each
	if fit := EnvironmentsThatFit(6 * gigabyte); fit != 9 {
		t.Fatalf("en 6 GB caben 9 entornos, y dijo %d", fit)
	}

	if fit := EnvironmentsThatFit(gigabyte / 2); fit != 0 {
		t.Fatalf("no puede caber un número negativo de entornos: %d", fit)
	}
}

func TestGigabytesAreRoundedNotTruncated(t *testing.T) {
	// 6.2 GB reported as "5" is the kind of small lie that makes somebody check the number
	// somewhere else and stop trusting the rest
	if rounded := Gigabytes(6*gigabyte + gigabyte/5); rounded != 6 {
		t.Fatalf("6,2 GB son 6 GB, y dijo %d", rounded)
	}

	if rounded := Gigabytes(5*gigabyte + 3*gigabyte/4); rounded != 6 {
		t.Fatalf("5,75 GB redondean a 6, y dijo %d", rounded)
	}
}

func TestComparingVersions(t *testing.T) {
	cases := []struct {
		version, target string
		atLeast         bool
	}{
		{"2.34.0", "2.0.0", true},
		{"2.0.0", "2.0.0", true},
		{"1.29.2", "2.0.0", false},
		{"2.4", "2.0.0", true},
		{"v2.10.0", "2.9.0", true},
		{"2.10.0", "2.9.0", true},
		{"2.24.0-desktop.1", "2.0.0", true},
	}

	for _, one := range cases {
		if got := VersionAtLeast(one.version, one.target); got != one.atLeast {
			t.Errorf("%q >= %q debería ser %v", one.version, one.target, one.atLeast)
		}
	}
}

func TestTheSummaryCountsWhatWasFound(t *testing.T) {
	diagnosis := Diagnosis{Checks: []Finding{
		{Severity: SeverityOK},
		{Severity: SeverityWarning},
		{Severity: SeverityError},
		{Severity: SeverityError},
	}}

	diagnosis.Summarise()

	if diagnosis.Summary.Total != 4 || diagnosis.Summary.OK != 1 ||
		diagnosis.Summary.Warnings != 1 || diagnosis.Summary.Errors != 2 {
		t.Fatalf("el resumen no cuadra: %+v", diagnosis.Summary)
	}
}
