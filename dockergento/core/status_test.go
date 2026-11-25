package core

import "testing"

func TestLastErrorLineIsTheLastLineThatReportsOne(t *testing.T) {
	cases := []struct {
		name string
		log  string
		want string
	}{
		{"an empty log", "", ""},
		{"nothing wrong", "ready to handle connections\nNOTICE: fpm is running\n", ""},
		{
			"nginx, the last of two",
			"2026/11/25 10:00:00 [error] 7#7: first\nok\n2026/11/25 10:01:00 [error] 7#7: second\nok again\n",
			"2026/11/25 10:01:00 [error] 7#7: second",
		},
		{"php-fpm's fatal", "[25-Nov-2026 10:00:00] PHP Fatal error:  Uncaught Exception\n", "[25-Nov-2026 10:00:00] PHP Fatal error:  Uncaught Exception"},
		{"mariadb's upper case", "2026-11-25 10:00:00 0 [ERROR] Aborted connection\n", "2026-11-25 10:00:00 0 [ERROR] Aborted connection"},
		{"a critical", "main.CRITICAL: something\n", "main.CRITICAL: something"},
		{"a word that only contains it is not one", "error_reporting = E_ALL\nlog_errors = On\nterror\n", ""},
		{"a warning is not an error", "WARNING: [pool www] server reached pm.max_children\n", ""},
		{"surrounding space is dropped", "   [error] spaced   \r\n", "[error] spaced"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := LastErrorLine(tt.log); got != tt.want {
				t.Fatalf("LastErrorLine(%q) = %q, want %q", tt.log, got, tt.want)
			}
		})
	}
}

// TestALongErrorLineIsCut: a stack trace on one line is kilobytes, and the line is a pointer to
// the log, not the log.
func TestALongErrorLineIsCut(t *testing.T) {
	long := "[error] "
	for len(long) < 1000 {
		long += "x"
	}

	got := LastErrorLine(long)

	if len([]rune(got)) != maxErrorLine+1 {
		t.Fatalf("len = %d, want %d and an ellipsis", len([]rune(got)), maxErrorLine)
	}
}
