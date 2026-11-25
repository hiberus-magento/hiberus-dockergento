package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// phpunitReport is the shape PHPUnit writes with --log-junit: suites inside suites, one per test
// class, and a failure's text starting with the test's name.
const phpunitReport = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="Unit" tests="5" assertions="5" errors="1" failures="1" skipped="1" time="0.250000">
    <testsuite name="Acme\Shop\Test\Unit\PriceTest" file="/var/www/html/app/code/Acme/Shop/Test/Unit/PriceTest.php" tests="4">
      <testcase name="testRounds" class="Acme\Shop\Test\Unit\PriceTest" classname="Acme.Shop.Test.Unit.PriceTest" file="/var/www/html/app/code/Acme/Shop/Test/Unit/PriceTest.php" line="12" assertions="1" time="0.100000"/>
      <testcase name="testDiscount" class="Acme\Shop\Test\Unit\PriceTest" classname="Acme.Shop.Test.Unit.PriceTest" file="/var/www/html/app/code/Acme/Shop/Test/Unit/PriceTest.php" line="20" assertions="1" time="0.050000">
        <failure type="PHPUnit\Framework\ExpectationFailedException">Acme\Shop\Test\Unit\PriceTest::testDiscount
Failed asserting that 9.0 matches expected 10.0.

/var/www/html/app/code/Acme/Shop/Test/Unit/PriceTest.php:24</failure>
      </testcase>
      <testcase name="testTax" class="Acme\Shop\Test\Unit\PriceTest" classname="Acme.Shop.Test.Unit.PriceTest" file="/x" line="30" assertions="0" time="0.050000">
        <error type="Error">Acme\Shop\Test\Unit\PriceTest::testTax
Error: Call to undefined method Acme\Shop\Model\Price::tax()</error>
      </testcase>
      <testcase name="testLater" class="Acme\Shop\Test\Unit\PriceTest" file="/x" line="40" assertions="0" time="0.000000">
        <skipped/>
      </testcase>
    </testsuite>
    <testsuite name="Acme\Shop\Test\Unit\CartTest" tests="1">
      <testcase name="testEmpty" class="Acme\Shop\Test\Unit\CartTest" file="/y" line="9" assertions="1" time="0.050000"/>
    </testsuite>
  </testsuite>
</testsuites>
`

func TestAPHPUnitReportIsCountedAndItsFailuresNamed(t *testing.T) {
	report, err := ParseJUnit([]byte(phpunitReport))
	if err != nil {
		t.Fatalf("ParseJUnit = %v", err)
	}

	want := TestReport{
		Tests: 5, Failures: 1, Errors: 1, Skipped: 1, Seconds: 0.25,
		Failed: []FailedTest{
			{Name: `Acme\Shop\Test\Unit\PriceTest::testDiscount`, Kind: "failure",
				Message: "Failed asserting that 9.0 matches expected 10.0.\n\n/var/www/html/app/code/Acme/Shop/Test/Unit/PriceTest.php:24"},
			{Name: `Acme\Shop\Test\Unit\PriceTest::testTax`, Kind: "error",
				Message: `Error: Call to undefined method Acme\Shop\Model\Price::tax()`},
		},
	}

	if diff := cmp.Diff(want, report); diff != "" {
		t.Fatalf("ParseJUnit (-want +got):\n%s", diff)
	}
}

// TestASingleSuiteAtTheTopIsAReportToo: other runners, and old PHPUnit with one suite, write a
// <testsuite> as the root.
func TestASingleSuiteAtTheTopIsAReportToo(t *testing.T) {
	report, err := ParseJUnit([]byte(`<testsuite name="x"><testcase name="a" classname="A" time="1.5"/></testsuite>`))
	if err != nil || report.Tests != 1 || report.Seconds != 1.5 || len(report.Failed) != 0 {
		t.Fatalf("ParseJUnit = %+v, %v; want one passing test", report, err)
	}

	if report.Failed == nil {
		t.Fatal("Failed is nil, want [] in the JSON")
	}
}

func TestWhatIsNotAReportIsAnError(t *testing.T) {
	for _, contents := range []string{"", "Fatal error: Class not found", "<html><body>no</body></html>", "<testsuites"} {
		if _, err := ParseJUnit([]byte(contents)); err == nil {
			t.Errorf("ParseJUnit(%q) = nil, want an error", contents)
		}
	}
}

// TestALongListOfFailuresIsCut: a broken bootstrap fails every test, and a thousand identical
// messages are a context window, not an answer.
func TestALongListOfFailuresIsCut(t *testing.T) {
	var report strings.Builder

	report.WriteString("<testsuites><testsuite>")

	for range 50 {
		report.WriteString(`<testcase name="t" class="C"><failure>C::t` + "\n" + strings.Repeat("m", 2000) + `</failure></testcase>`)
	}

	report.WriteString("</testsuite></testsuites>")

	parsed, err := ParseJUnit([]byte(report.String()))
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Failures != 50 || len(parsed.Failed) != maxFailedTests {
		t.Fatalf("failures %d, listed %d; want 50 counted and %d listed", parsed.Failures, len(parsed.Failed), maxFailedTests)
	}

	if len([]rune(parsed.Failed[0].Message)) > maxFailureMessage+1 {
		t.Fatalf("message is %d long, want it cut at %d", len(parsed.Failed[0].Message), maxFailureMessage)
	}
}
