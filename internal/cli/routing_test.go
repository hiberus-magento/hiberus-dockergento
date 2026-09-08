package cli

import (
	"io"
	"testing"
)

//
// What the router does with the docker tools, as opposed to what their handlers ask. A command
// that is not yet a case in run.go's switch falls through to the legacy runner, which answering(t)
// pins to a fresh temporary directory that has no bin/run — so an unrouted call fails with 3 and
// records nothing, rather than reaching a developer's own shell tree.
//

func TestTheRouterAnswersForTheDockerTools(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"docker-stop-all", []string{"docker-stop-all", "--yes"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			Run(tt.args, io.Discard, io.Discard)

			//
			// The legacy fallback (run.go's final Shell() call) reaches the real engine(), never
			// newEngine — so as long as the command falls through unrouted, the fake this test
			// substituted for newEngine is asked nothing at all. A case line in the switch is what
			// makes the handler reach the fake instead.
			//
			if len(fake.calls) == 0 {
				t.Fatalf("%v asked the fake engine nothing, want the router to reach the handler "+
					"instead of falling through to the legacy runner", tt.args)
			}
		})
	}
}
