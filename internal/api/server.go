// Package api is a driving adapter: it turns HTTP requests into the same calls the command line
// makes, and their answers into the same documents it prints.
//
// The same calls, not similar ones. Every handler here is one method on the engine, which is what
// the separation was for: a dashboard that reimplemented "what is running" would answer something
// slightly different from `hm list` the first time either changed.
package api

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//go:embed dashboard.html
var pages embed.FS

// Server answers over HTTP what the tool answers in a terminal.
type Server struct {
	Engine *dockergento.Engine

	// Token is required on every request. It is not a nicety: this API reads database
	// credentials and stops environments, and a page on the internet can make a browser send
	// requests to a port on this machine. Without something the page cannot know, any site
	// somebody visits could stop their work.
	Token string

	// Version is what the health check reports, so a dashboard left open in a tab can tell it is
	// talking to a binary that has since been replaced.
	Version string
}

// NewToken makes one. Long enough that guessing it is not a strategy.
func NewToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

// Handler is everything the server answers.
func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/environments", s.environments)
	mux.HandleFunc("GET /api/project", s.project)
	mux.HandleFunc("GET /api/project/doctor", s.doctor)
	mux.HandleFunc("POST /api/project/start", s.start)
	mux.HandleFunc("POST /api/project/stop", s.stop)
	mux.HandleFunc("POST /api/project/restart", s.restart)
	mux.HandleFunc("GET /", s.dashboard)

	return s.guard(mux)
}

// guard is the only security this has, and it is deliberately two plain rules rather than a
// framework.
//
// The first is the token. The second is the Host header: a name on the internet can be pointed at
// 127.0.0.1, and then a page served from it is same-origin with this server as far as the browser
// is concerned. Refusing any Host that is not loopback is what stops that, and it costs one
// comparison.
func (s Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !loopbackHost(request.Host) {
			http.Error(writer, "this server only answers to localhost", http.StatusForbidden)

			return
		}

		if s.Token != "" && token(request) != s.Token {
			http.Error(writer, "wrong or missing token", http.StatusUnauthorized)

			return
		}

		writer.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(writer, request)
	})
}

func loopbackHost(host string) bool {
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}

	if name == "localhost" {
		return true
	}

	address := net.ParseIP(strings.Trim(name, "[]"))

	return address != nil && address.IsLoopback()
}

// token is read from the header or from the query, because the dashboard is opened by pasting a
// link and a browser cannot be told to send a header.
func token(request *http.Request) string {
	if header := request.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}

	return request.URL.Query().Get("token")
}

func (s Server) health(writer http.ResponseWriter, _ *http.Request) {
	answer(writer, "health", map[string]any{"version": s.Version}, nil)
}

func (s Server) environments(writer http.ResponseWriter, _ *http.Request) {
	environments, err := s.Engine.Environments()
	if err != nil {
		answer(writer, "list", nil, err)

		return
	}

	// The same document `hm list --json` prints, down to the wrapper: a reader should not have to
	// know which door the answer came through, and "the array, but wrapped over there" is exactly
	// the kind of difference that costs somebody an afternoon
	answer(writer, "list", map[string]any{
		"environments": environments,
		"count":        len(environments),
	}, nil)
}

func (s Server) project(writer http.ResponseWriter, request *http.Request) {
	root := request.URL.Query().Get("root")
	if root == "" {
		answer(writer, "describe", nil, errors.New("which project: pass ?root=<directory>"))

		return
	}

	// The credentials are asked for, never volunteered — the same rule the command line follows,
	// and the reason a dashboard left open does not have a password on the screen
	description, err := s.Engine.Describe(root, request.URL.Query().Get("secrets") == "true")

	answer(writer, "describe", description, err)
}

func (s Server) doctor(writer http.ResponseWriter, request *http.Request) {
	root := request.URL.Query().Get("root")
	if root == "" {
		answer(writer, "doctor", nil, errors.New("which project: pass ?root=<directory>"))

		return
	}

	diagnosis, err := s.Engine.Diagnose(root, request.URL.Query().Get("only"))

	answer(writer, "doctor", diagnosis, err)
}

// operation is what a request to change something carries.
type operation struct {
	Root       string   `json:"root"`
	Services   []string `json:"services"`
	StopOthers bool     `json:"stop_others"`
	Snapshot   bool     `json:"snapshot"`
}

func (s Server) start(writer http.ResponseWriter, request *http.Request) {
	asked, err := asked(request)
	if err != nil {
		answer(writer, "start", nil, err)

		return
	}

	err = s.Engine.Start(asked.Root, dockergento.StartOptions{
		Services:   asked.Services,
		StopOthers: asked.StopOthers,
	})

	answer(writer, "start", map[string]any{"root": asked.Root}, err)
}

func (s Server) stop(writer http.ResponseWriter, request *http.Request) {
	asked, err := asked(request)
	if err != nil {
		answer(writer, "stop", nil, err)

		return
	}

	err = s.Engine.Stop(asked.Root, asked.Services, asked.Snapshot)

	answer(writer, "stop", map[string]any{"root": asked.Root}, err)
}

func (s Server) restart(writer http.ResponseWriter, request *http.Request) {
	asked, err := asked(request)
	if err != nil {
		answer(writer, "restart", nil, err)

		return
	}

	err = s.Engine.Restart(asked.Root, asked.Services)

	answer(writer, "restart", map[string]any{"root": asked.Root}, err)
}

func asked(request *http.Request) (operation, error) {
	var wanted operation

	if err := json.NewDecoder(request.Body).Decode(&wanted); err != nil {
		return operation{}, fmt.Errorf("the request body is not the JSON this expects: %w", err)
	}

	if wanted.Root == "" {
		return operation{}, errors.New("which project: the body needs a root")
	}

	return wanted, nil
}

func (s Server) dashboard(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)

		return
	}

	page, err := pages.ReadFile("dashboard.html")
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)

		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Write(page) //nolint:errcheck
}

func answer(writer http.ResponseWriter, command string, data any, err error) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err == nil {
		write(writer, http.StatusOK, contract.Success(command, data))

		return
	}

	//
	// A refusal is not a failure of the server, and saying so matters: "this port is taken by
	// another environment" is something the person reading can act on, and a 500 tells them
	// nothing except to try again.
	//
	status := http.StatusBadRequest

	var refusal core.Refusal
	if errors.As(err, &refusal) {
		status = http.StatusConflict
	}

	write(writer, status, contract.FailureFrom(command, err, contract.ExitError, "failed"))
}

func write(writer http.ResponseWriter, status int, body contract.Envelope) {
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(body) //nolint:errcheck
}
