package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hm "github.com/hiberus-magento/hiberus-dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
)

// posted sends body to path on a server with a real engine, from localhost and with no token, and
// answers the status and the envelope.
func posted(t *testing.T, path, body string) (int, map[string]any) {
	t.Helper()

	server := Server{Engine: dockergento.New(dockergento.Options{
		Assets:   hm.Assets(),
		Binary:   "hm",
		StateDir: t.TempDir(),
	})}

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Host = "127.0.0.1:8080"
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)

	envelope := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("%s answered %q, not an envelope: %v", path, recorder.Body.String(), err)
	}

	return recorder.Code, envelope
}

// TestStopAndRestartAnswerAsTheCommandLineDoes: what the engine answers is passed on, and a body
// that is not the JSON expected is a 400, for both operations. A directory with no environment
// fails at reading its compose files, before anything reaches Docker: a failure, so a 400.
func TestStopAndRestartAnswerAsTheCommandLineDoes(t *testing.T) {
	for _, operation := range []string{"stop", "restart"} {
		t.Run(operation, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", "unix:///nonexistent")
			root := t.TempDir()
			path := "/api/project/" + operation

			status, envelope := posted(t, path, `{"root": "`+root+`", "services": ["phpfpm"], "snapshot": false}`)
			fault, _ := envelope["error"].(map[string]any)

			message, _ := fault["message"].(string)
			if status != http.StatusBadRequest || envelope["ok"] != false || envelope["command"] != operation ||
				fault["type"] != "failed" || message != "no compose file to read in "+root {
				t.Fatalf("%s of an empty directory = %d %v, want 400 failed, no compose file", operation, status, envelope)
			}

			status, envelope = posted(t, path, `not json`)
			if status != http.StatusBadRequest || envelope["ok"] != false || envelope["command"] != operation {
				t.Fatalf("%s with a broken body = %d %v, want 400", operation, status, envelope)
			}
		})
	}
}
