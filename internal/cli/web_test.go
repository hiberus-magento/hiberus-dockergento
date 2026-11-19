package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"
)

//
// What web does that can be pinned without a daemon and without starting anything: how it reads
// its arguments, and how it answers from the state file a running server leaves in ~/.hm/web.json.
// HOME is a temporary directory in every test, so that file is the test's own and the machine's
// real one, if any, is never read, written or removed.
//
// Never a live state file whose pid is somebody else's, and never `down` against a live pid: down
// sends SIGTERM to whatever that pid is. The only live pid used is this test process's own, and
// only for answers that read it.
//

// webEnvelope is the part of the contract envelope these tests read.
type webEnvelope struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data"`
	Error struct {
		Code int    `json:"code"`
		Type string `json:"type"`
	} `json:"error"`
}

func decodeWeb(t *testing.T, raw []byte) webEnvelope {
	t.Helper()

	var envelope webEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("not a JSON envelope: %v\n%s", err, raw)
	}

	return envelope
}

// webHome points HOME at a fresh directory and returns where web keeps its state in it.
func webHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	return filepath.Join(home, ".hm")
}

func writeWebState(t *testing.T, dir string, state running) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "web.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// deadPID is a pid no process can have: above every system's pid limit, so kill answers ESRCH.
//
// Not the pid of a child that has already exited: the system may hand that pid to another process
// before the test reads it, and `down` sends SIGTERM to a pid it believes is alive.
func deadPID(t *testing.T) int {
	t.Helper()

	if alive(math.MaxInt32) {
		t.Fatalf("pid %d answers signal 0; it cannot stand in for a dead server", math.MaxInt32)
	}

	return math.MaxInt32
}

// TestWebRefusesWhatItDoesNotKnow pins the argument parsing (web.go:46-72): every malformed
// invocation is a usage error, and none of them touches the state directory.
func TestWebRefusesWhatItDoesNotKnow(t *testing.T) {
	cases := []struct {
		name string
		args []string
		kind string
	}{
		{"an unknown option", []string{"--nonsense"}, "invalid_argument"},
		{"an unknown action", []string{"restart"}, "invalid_argument"},
		{"--port with nothing after it", []string{"--port"}, "missing_value"},
		{"--port with something that is not a number", []string{"--port", "eighty"}, "invalid_argument"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := webHome(t)

			var stdout, stderr bytes.Buffer
			code := web(tt.args, &stdout, &stderr, true)

			if code != exitUsage {
				t.Fatalf("web %v = %d, want exitUsage", tt.args, code)
			}

			if envelope := decodeWeb(t, stderr.Bytes()); envelope.Error.Type != tt.kind {
				t.Errorf("error type = %q, want %q", envelope.Error.Type, tt.kind)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}

			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("the state directory was touched: %v", err)
			}
		})
	}
}

// TestWebStatusWithNothingRunning: no state file is not running, and so is a state file whose
// process is gone.
func TestWebStatusWithNothingRunning(t *testing.T) {
	t.Run("no state file", func(t *testing.T) {
		webHome(t)

		var stdout bytes.Buffer
		if code := web([]string{"status"}, &stdout, &bytes.Buffer{}, true); code != exitOK {
			t.Fatalf("web status = %d, want exitOK", code)
		}

		envelope := decodeWeb(t, stdout.Bytes())
		if !envelope.OK || envelope.Data["running"] != false {
			t.Errorf("status = %+v, want ok and running false", envelope)
		}

		if _, present := envelope.Data["url"]; present {
			t.Errorf("a server that is not running has no address: %+v", envelope.Data)
		}
	})

	t.Run("a stale state file", func(t *testing.T) {
		dir := webHome(t)
		writeWebState(t, dir, running{PID: deadPID(t), Port: 8420, Token: "stale"})

		var stdout bytes.Buffer
		if code := web([]string{"status"}, &stdout, &bytes.Buffer{}, true); code != exitOK {
			t.Fatalf("web status = %d, want exitOK", code)
		}

		if envelope := decodeWeb(t, stdout.Bytes()); envelope.Data["running"] != false {
			t.Errorf("status with a dead pid = %+v, want running false", envelope.Data)
		}
	})
}

// TestWebStatusReadsARunningServer: a state file whose process is alive is reported with
// everything a client needs to reach it.
func TestWebStatusReadsARunningServer(t *testing.T) {
	dir := webHome(t)
	writeWebState(t, dir, running{PID: os.Getpid(), Port: 8433, Token: "abc", Started: "2026-11-19 10:00"})

	var stdout bytes.Buffer
	if code := web([]string{"status"}, &stdout, &bytes.Buffer{}, true); code != exitOK {
		t.Fatalf("web status = %d, want exitOK", code)
	}

	data := decodeWeb(t, stdout.Bytes()).Data
	want := map[string]any{
		"running": true,
		"port":    float64(8433),
		"pid":     float64(os.Getpid()),
		"url":     "http://127.0.0.1:8433/?token=abc",
		"started": "2026-11-19 10:00",
	}

	for key, value := range want {
		if data[key] != value {
			t.Errorf("status %s = %v, want %v", key, data[key], value)
		}
	}
}

// TestWebDownWithNothingRunning: down with a stale state file stops nothing, says so, and clears
// the file so the next status does not have to tell the difference again.
func TestWebDownWithNothingRunning(t *testing.T) {
	dir := webHome(t)
	writeWebState(t, dir, running{PID: deadPID(t), Port: 8420, Token: "stale"})

	var stdout bytes.Buffer
	if code := web([]string{"down"}, &stdout, &bytes.Buffer{}, true); code != exitOK {
		t.Fatalf("web down = %d, want exitOK", code)
	}

	data := decodeWeb(t, stdout.Bytes()).Data
	if data["running"] != false || data["stopped"] != false {
		t.Errorf("down = %+v, want running false and stopped false", data)
	}

	if _, err := os.Stat(filepath.Join(dir, "web.json")); !os.IsNotExist(err) {
		t.Errorf("the stale state file is still there: %v", err)
	}
}

// TestWebUpFindsTheOneAlreadyRunning: up with a live server answers with it instead of starting
// a second one. The live pid is this test's own, so nothing is started and nothing is signalled.
func TestWebUpFindsTheOneAlreadyRunning(t *testing.T) {
	dir := webHome(t)
	writeWebState(t, dir, running{PID: os.Getpid(), Port: 8433, Token: "abc"})

	var stdout bytes.Buffer
	if code := web(nil, &stdout, &bytes.Buffer{}, true); code != exitOK {
		t.Fatalf("web = %d, want exitOK", code)
	}

	data := decodeWeb(t, stdout.Bytes()).Data
	if data["running"] != true || data["url"] != "http://127.0.0.1:8433/?token=abc" {
		t.Errorf("up = %+v, want the running server's address", data)
	}

	if _, err := os.Stat(filepath.Join(dir, "web.log")); !os.IsNotExist(err) {
		t.Errorf("a second server was started: its log exists (%v)", err)
	}
}

// TestWebInTheForegroundRefusesATakenPort: a port somebody else holds is a refusal with the next
// one as the way out, before any state is written.
func TestWebInTheForegroundRefusesATakenPort(t *testing.T) {
	dir := webHome(t)

	holder, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()

	port := holder.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	code := web([]string{"--foreground", "--port", fmt.Sprint(port)}, &stdout, &stderr, true)

	if code != exitBlocked {
		t.Fatalf("web --foreground on a taken port = %d, want exitBlocked", code)
	}

	if envelope := decodeWeb(t, stderr.Bytes()); envelope.Error.Type != "port_taken" {
		t.Errorf("error type = %q, want port_taken", envelope.Error.Type)
	}

	if _, err := os.Stat(filepath.Join(dir, "web.json")); !os.IsNotExist(err) {
		t.Errorf("state was written for a server that never listened: %v", err)
	}
}
