package core

import (
	"encoding/xml"
	"errors"
	"strings"
)

// TestReport is what a JUnit report says about a test run: how many ran, failed, errored and were
// skipped, how long they took, and which failed and why.
type TestReport struct {
	Tests    int          `json:"tests"`
	Failures int          `json:"failures"`
	Errors   int          `json:"errors"`
	Skipped  int          `json:"skipped"`
	Seconds  float64      `json:"seconds"`
	Failed   []FailedTest `json:"failed"`
}

// FailedTest is one test that failed (an assertion) or errored (an exception), with its message.
type FailedTest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

const (
	// maxFailedTests is how many failures are listed: a broken bootstrap fails every test, and a
	// thousand identical messages are a context window, not an answer. They are all counted.
	maxFailedTests = 20

	// maxFailureMessage is how much of each message is kept.
	maxFailureMessage = 1000
)

type junitSuite struct {
	XMLName xml.Name
	Suites  []junitSuite `xml:"testsuite"`
	Cases   []junitCase  `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Class     string        `xml:"class,attr"`
	Classname string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure"`
	Error     *junitMessage `xml:"error"`
	Skipped   *struct{}     `xml:"skipped"`
}

type junitMessage struct {
	Text string `xml:",chardata"`
}

// ParseJUnit reads a JUnit XML report, as PHPUnit writes it with --log-junit: a <testsuites> or a
// single <testsuite> at the top, with suites nested in it.
//
// The test cases are counted rather than the suites' attributes read: PHPUnit and other runners
// do not agree on which attributes a suite carries, and every one of them writes the cases.
func ParseJUnit(contents []byte) (TestReport, error) {
	var root junitSuite

	if err := xml.Unmarshal(contents, &root); err != nil {
		return TestReport{}, err
	}

	if root.XMLName.Local != "testsuites" && root.XMLName.Local != "testsuite" {
		return TestReport{}, errors.New("not a JUnit report: its root is <" + root.XMLName.Local + ">")
	}

	report := TestReport{Failed: []FailedTest{}}
	report.add(root)

	return report, nil
}

func (r *TestReport) add(suite junitSuite) {
	for _, one := range suite.Cases {
		r.Tests++
		r.Seconds += one.Time

		switch {
		case one.Failure != nil:
			r.Failures++
			r.failed(one, "failure", one.Failure.Text)
		case one.Error != nil:
			r.Errors++
			r.failed(one, "error", one.Error.Text)
		case one.Skipped != nil:
			r.Skipped++
		}
	}

	for _, inner := range suite.Suites {
		r.add(inner)
	}
}

func (r *TestReport) failed(one junitCase, kind, text string) {
	if len(r.Failed) >= maxFailedTests {
		return
	}

	class := one.Class
	if class == "" {
		class = one.Classname
	}

	name := one.Name
	if class != "" {
		name = class + "::" + one.Name
	}

	// PHPUnit starts the message with the test's own name, which is already in Name
	message := strings.TrimSpace(text)
	if first, rest, found := strings.Cut(message, "\n"); found && strings.TrimSpace(first) == name {
		message = strings.TrimSpace(rest)
	}

	if runes := []rune(message); len(runes) > maxFailureMessage {
		message = string(runes[:maxFailureMessage]) + "…"
	}

	r.Failed = append(r.Failed, FailedTest{Name: name, Kind: kind, Message: message})
}
