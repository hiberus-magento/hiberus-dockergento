package core

import "testing"

// The memory verdict is the one piece of the diagnosis with real judgement in it, and it is pure
// so that the judgement can be exercised without a daemon: the numbers below are the machines
// this was written for, not invented ones.

func TestAVirtualMachineTooSmallForOneStack(t *testing.T) {
	verdict := VMMemoryVerdict(2*gigabyte, 48*gigabyte)

	if verdict != "small" {
		t.Fatalf("verdict for 2 GB = %q, want it to say a full stack does not fit", verdict)
	}
}

func TestALaptopWithPlentyAndAVirtualMachineWithout(t *testing.T) {
	// The case this check exists for: 48 GB of machine, 6 of Docker, and environments that will
	// not start on a laptop with memory to spare
	verdict := VMMemoryVerdict(6*gigabyte, 48*gigabyte)

	if verdict != "cramped" {
		t.Fatalf("verdict for 6 GB of 48 = %q, want it to say the VM is the narrow part", verdict)
	}
}

func TestASmallMachineIsNotCramped(t *testing.T) {
	// 6 of 8 is the hardware, not a setting somebody forgot, and saying otherwise would be a
	// warning nobody can act on
	verdict := VMMemoryVerdict(6*gigabyte, 8*gigabyte)

	if verdict != "fine" {
		t.Fatalf("verdict for 6 GB of 8 = %q, want it to say that is what the machine has", verdict)
	}
}

func TestNoAnswerIsNotABadAnswer(t *testing.T) {
	if verdict := VMMemoryVerdict(0, 48*gigabyte); verdict != "unknown" {
		t.Fatalf("verdict with nothing measured = %q, want none", verdict)
	}
}

func TestHowManyEnvironmentsFit(t *testing.T) {
	// Measured on the machine this was written on: eight environments in a 5.9 GiB virtual
	// machine, at about 550 MB each
	if fit := EnvironmentsThatFit(6 * gigabyte); fit != 9 {
		t.Fatalf("environments that fit in 6 GB = %d, want 9", fit)
	}

	if fit := EnvironmentsThatFit(gigabyte / 2); fit != 0 {
		t.Fatalf("environments that fit = %d, want not less than zero", fit)
	}
}

func TestGigabytesAreRoundedNotTruncated(t *testing.T) {
	// 6.2 GB reported as "5" is the kind of small lie that makes somebody check the number
	// somewhere else and stop trusting the rest
	if rounded := Gigabytes(6*gigabyte + gigabyte/5); rounded != 6 {
		t.Fatalf("6.2 GB rounded = %d, want 6", rounded)
	}

	if rounded := Gigabytes(5*gigabyte + 3*gigabyte/4); rounded != 6 {
		t.Fatalf("5.75 GB rounded = %d, want 6", rounded)
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
			t.Errorf("atLeast(%q, %q) = %v, want %v", one.version, one.target, !one.atLeast, one.atLeast)
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
		t.Fatalf("summary = %+v, want the counts to add up", diagnosis.Summary)
	}
}
