package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// Under --json stdout is the envelope and nothing else. What bin/magento says while varnish is
// switched — "Changed cache status", "Flushed cache types" — is the container's stdout, which is
// the process's own, so it is sent to the container's stderr and asked for without a terminal:
// a pseudo-terminal merges both streams into stdout, whatever the command redirects.
//

func TestVarnishUnderJSONSendsMagentoOutputToStderr(t *testing.T) {
	cases := []struct {
		command string
		want    [][]string
	}{
		{"varnish-on", [][]string{
			{"sh", "-c", "bin/magento cache:enable full_page >&2"},
		}},
		{"varnish-off", [][]string{
			{"sh", "-c", "bin/magento cache:disable full_page >&2"},
			{"sh", "-c", "rm -rf " + strings.Join(generated, " ")},
			{"sh", "-c", "php ./bin/magento cache:clean >&2"},
		}},
	}

	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			fake := shopForEnvelopes(t)

			var stdout, stderr bytes.Buffer
			if code := Run([]string{tt.command, "--json"}, &stdout, &stderr); code != exitOK {
				t.Fatalf("hm %s --json = %d, want exitOK\nstderr: %s", tt.command, code, stderr.String())
			}

			var got [][]string

			for _, asked := range fake.calls {
				if asked.Method != "Exec" || asked.Service != phpService {
					continue
				}

				got = append(got, asked.Command)

				if asked.Options.Tty {
					t.Errorf("%v asked for a terminal under --json, which merges stderr into stdout", asked.Command)
				}
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("hm %s --json ran in phpfpm (-want +got):\n%s", tt.command, diff)
			}

			if count := strings.Count(stdout.String(), "\"schema_version\""); count != 1 {
				t.Fatalf("stdout holds %d envelopes, want 1:\n%s", count, stdout.String())
			}
		})
	}
}
