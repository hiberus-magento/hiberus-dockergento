package dockergento

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

	// Mailhog answers HEAD with a 404 and GET with its page: a server that refuses the method is
	// asked again the way a browser asks, and what that says is the answer
	for _, refused := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		t.Run(fmt.Sprintf("a HEAD answered %d is asked again with a GET", refused), func(t *testing.T) {
			var methods []string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)

				if r.Method == http.MethodHead {
					w.WriteHeader(refused)

					return
				}

				io.WriteString(w, "<html>mailhog</html>") //nolint:errcheck
			}))
			defer server.Close()

			code, err := urlProbe{timeout: 2 * time.Second}.Answer(server.URL)
			if err != nil || code != http.StatusOK {
				t.Fatalf("Answer() = %d, %v; want the 200 the GET was given", code, err)
			}

			if diff := cmp.Diff([]string{http.MethodHead, http.MethodGet}, methods); diff != "" {
				t.Errorf("methods (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("the GET follows no redirect either, and its own refusal is the answer", func(t *testing.T) {
		followed := false

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/elsewhere":
				followed = true
			case r.Method == http.MethodHead:
				w.WriteHeader(http.StatusMethodNotAllowed)
			default:
				http.Redirect(w, r, "/elsewhere", http.StatusFound)
			}
		}))
		defer server.Close()

		code, err := urlProbe{timeout: 2 * time.Second}.Answer(server.URL)
		if err != nil || code != http.StatusFound || followed {
			t.Fatalf("Answer() = %d, %v (followed %v); want the GET's 302, not followed", code, err, followed)
		}
	})

	t.Run("a HEAD answered otherwise is not asked twice", func(t *testing.T) {
		asked := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			asked++

			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		code, err := urlProbe{timeout: 2 * time.Second}.Answer(server.URL)
		if err != nil || code != http.StatusInternalServerError || asked != 1 {
			t.Fatalf("Answer() = %d, %v after %d requests; want the 500, asked once", code, err, asked)
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
