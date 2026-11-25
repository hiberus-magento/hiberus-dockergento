package dockergento

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

//
// The URL probe hm status asks each address with, against real servers on localhost: what it
// answers is the decision status reports, so it is proved on the wire rather than on a stand-in.
//

func TestTheURLProbeSaysWhatAnswered(t *testing.T) {
	t.Run("a redirect is the answer, not followed", func(t *testing.T) {
		followed := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/elsewhere" {
				followed = true

				return
			}

			if r.Method != http.MethodHead {
				t.Errorf("method = %s, want HEAD: nothing is asked for but the answer", r.Method)
			}

			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}))
		defer server.Close()

		code, err := urlProbe{timeout: 2 * time.Second}.Answer(server.URL + "/admin")
		if err != nil || code != http.StatusFound {
			t.Fatalf("Answer() = %d, %v; want 302", code, err)
		}

		if followed {
			t.Error("the redirect was followed")
		}
	})

	t.Run("a certificate nobody here trusts is still an answer", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		code, err := urlProbe{timeout: 2 * time.Second}.Answer(server.URL)
		if err != nil || code != http.StatusServiceUnavailable {
			t.Fatalf("Answer() = %d, %v; want the 503 the self-signed server gave", code, err)
		}
	})

	t.Run("a server that does not answer in time is an error", func(t *testing.T) {
		release := make(chan struct{})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			<-release
		}))
		defer server.Close()
		defer close(release)

		started := time.Now()

		code, err := urlProbe{timeout: 100 * time.Millisecond}.Answer(server.URL)
		if err == nil {
			t.Fatalf("Answer() = %d, nil; want the timeout said", code)
		}

		if waited := time.Since(started); waited > 5*time.Second {
			t.Errorf("waited %s, want the probe's own timeout to end it", waited)
		}
	})
}
