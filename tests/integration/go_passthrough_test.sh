#!/usr/bin/env bash
#
# The binary's own answers. It answers every command itself; the shell entry point it used to be
# compared with is gone.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"; hm_test_home_cleanup' EXIT

if ! command -v go >/dev/null 2>&1; then
    echo "  - skipped: go is not installed"
    echo "RESULT 0 0"
    exit 0
fi

#
# The build cache goes in the lab, not in the throwaway HOME: this suite replaces the harness's
# own cleanup trap with its own, so anything left in HOME outlives the run — once per run, for
# ever. The module cache stays where it is, or every run downloads the dependency tree again.
#
export GOCACHE="$LAB/go-build"

( cd "$COMMAND_BIN_DIR" && go build -o "$GO_BINARY" ./cmd/hm ) >/dev/null 2>&1 || {
    echo "  - skipped: the binary does not build here"
    echo "RESULT 0 0"
    exit 0
}

PROJECT="hm-go-selftest"
DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
printf 'services:\n  phpfpm:\n    image: alpine:latest\n' > "$DIR/docker-compose.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "go.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

# ---------------------------------------------------------------- the version

( cd "$DIR" && "$GO_BINARY" --version >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
test_case "--version answers, and succeeds"
assert_equals "0" "$GO_STATUS"
assert_json_field "$(cat "$LAB/go.out")" '.command' "version"

# ---------------------------------------------------------------- ported: describe
#
# The richest document the tool produces, and the command run most often. This used to compare
# the shell and Go implementations directly — the whole document, with and without secrets, and
# the table with colour turned off — which caught a field nobody wrote an assertion for and a
# rendering that drifted a single byte, because the two implementations had to agree on everything
# to pass, not only on what somebody checked.
#
# describe.sh is retired in this same change, so there is no shell answer left to compare against.
# What follows checks the Go answer against a fixed oracle instead — the document's own declared
# sections, and the fixture's own configuration — which proves the sections and fields somebody
# named still exist, but not that the two implementations ever agreed on everything else: an
# undocumented field going missing, or the --no-json table's column widths and spacing drifting,
# would no longer be caught by anything here.

( cd "$DIR" && "$GO_BINARY" describe --json >"$LAB/go.out" 2>"$LAB/go.err" )
test_case "the document still carries every documented section"
assert_json_field "$(cat "$LAB/go.out")" \
    '.data | has("project") and has("magento") and has("services") and has("paths") and has("tooling")' \
    "true"

( cd "$DIR" && "$GO_BINARY" describe --json --with-secrets >"$LAB/go.out" 2>"$LAB/go.err" )
test_case "and with the credentials too, when they are asked for"
assert_json_field "$(cat "$LAB/go.out")" '.data.credentials.database | has("password")' "true"

test_case "which are never volunteered"
( cd "$DIR" && "$GO_BINARY" describe --json >"$LAB/go.out" 2>"$LAB/go.err" )
assert_equals "null" "$(jq -r '.data.credentials // "null"' < "$LAB/go.out")"

test_case "and the table still names its sections"
( cd "$DIR" && NO_COLOR=1 "$GO_BINARY" --no-json describe >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Environment"
assert_contains "$(cat "$LAB/go.out")" "URLs"
assert_contains "$(cat "$LAB/go.out")" "Magento"
assert_contains "$(cat "$LAB/go.out")" "Services"
assert_contains "$(cat "$LAB/go.out")" "Paths"

( cd "$DIR" && "$GO_BINARY" describe --nonsense >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
test_case "an option nobody declared is its own usage error"
assert_equals "2" "$GO_STATUS"
assert_json_field "$(cat "$LAB/go.err")" '.error.type' "invalid_argument"

# ---------------------------------------------------------------- ported: list
#
# The first command that stopped going through the shell implementation. This used to compare the
# whole document and the whole table against the shell one, because the moment the two were
# allowed to differ a little, nobody could tell a port from a regression.
#
# list.sh is retired in this same change, so there is no shell document or shell table left to
# compare against. introspection_test.sh:37-52 already proves the full environment shape, so what
# is left here is the envelope fields nothing else names, plus the refusal — not a duplicate of
# that suite and not a replacement for the comparison: an unnoticed field, or the table rendering
# drifting a column width, is no longer caught by anything.

( cd "$DIR" && "$GO_BINARY" list --json >"$LAB/go.out" 2>"$LAB/go.err" )
test_case "the envelope carries the documented shape"
assert_json_field "$(cat "$LAB/go.out")" '.schema_version' "1"
assert_json_field "$(cat "$LAB/go.out")" '.command' "list"
assert_json_field "$(cat "$LAB/go.out")" '.data.count | type' "number"

( cd "$DIR" && "$GO_BINARY" list --nonsense >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
test_case "and an option nobody declared is its own usage error"
assert_equals "2" "$GO_STATUS"
assert_json_field "$(cat "$LAB/go.err")" '.error.type' "invalid_argument"

#
# The tool's own flags are accepted before the command name as well as after it, which is what the
# shell implementation does. Consuming them only after meant `hm --no-json describe` never reached
# the Go implementation at all: it fell through to the shell one, silently, and looked exactly
# like it had worked — including to the tests that were comparing the two.
#
test_case "a global flag before the command still reaches the Go implementation"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" --no-json version >/dev/null 2>&1; echo $? )"

test_case "and it decides the format, wherever it is written"
assert_equals "$( cd "$DIR" && "$GO_BINARY" --json describe )" "$( cd "$DIR" && "$GO_BINARY" describe --json )"

#
# Piped output is being read by a program, and a program reading a table of dashes breaks the
# first time a column widens. Both implementations answer JSON when nobody is watching.
#
test_case "both default to JSON when the output is not a terminal"
assert_equals "1" "$( cd "$DIR" && "$GO_BINARY" list | jq -r '.schema_version' )"

# ---------------------------------------------------------------- ported: doctor
#
# Seventeen checks, of which five ask Docker and four look at the host. doctor has no shell twin
# left to compare with; what a port of it can lose is a single check nobody notices missing, so the
# count and the order are what is held here. What each check decides is pinned in Go
# (dockergento/app/checks_test.go, doctor_test.go).

( cd "$DIR" && "$GO_BINARY" doctor --json >"$LAB/go.out" 2>&1 )
test_case "inside a project the diagnosis reports all seventeen checks"
assert_equals "17" "$(jq '.data.checks | length' < "$LAB/go.out")"

test_case "and in the same order every time"
( cd "$DIR" && "$GO_BINARY" doctor --json >"$LAB/go.again" 2>&1 )
assert_equals "$(jq -r '.data.checks[].id' < "$LAB/go.out")" "$(jq -r '.data.checks[].id' < "$LAB/go.again")"

test_case "and the text report has a line for each of them"
( cd "$DIR" && NO_COLOR=1 "$GO_BINARY" --no-json doctor >"$LAB/go.txt" 2>&1 )
for id in $(jq -r '.data.checks[].id' < "$LAB/go.out"); do
    assert_contains "$(cat "$LAB/go.txt")" "$id"
done

( cd "$DIR" && "$GO_BINARY" doctor --json --only=ports >"$LAB/go.out" 2>&1 )
test_case "one check on its own is that check and nothing else"
assert_equals "ports" "$(jq -r '[.data.checks[].id] | join(" ")' < "$LAB/go.out")"

#
# Outside a project the diagnosis still runs and answers about the machine — which is the question
# somebody has when nothing works anywhere.
#
test_case "outside a project it answers about the machine"
( cd "$LAB" && "$GO_BINARY" doctor --json >"$LAB/go.out" 2>&1 )
assert_equals "7" "$(jq '.data.checks | length' < "$LAB/go.out")"

test_case "and reports no project check there"
assert_equals "" "$(jq -r '[.data.checks[] | select(.scope == "project")] | .[]' < "$LAB/go.out")"

#
# The fingerprint is the one value in the diagnosis that has to agree with something already
# written down: the digest `hm ai-context` leaves in AGENTS.md is what the doctor's check compares
# against. A jq newline the port dropped would make every
# generated context on every project read as stale.
#
( cd "$DIR" && "$GO_BINARY" ai-context >/dev/null 2>&1 )

test_case "the Go check recognises the context the shell command generated"
( cd "$DIR" && "$GO_BINARY" doctor --json --only=agent-context >"$LAB/go.out" 2>&1 )
assert_equals "ok" "$(jq -r '.data.checks[0].severity' < "$LAB/go.out")"

test_case "and notices when the project moved on"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "otro.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"
( cd "$DIR" && "$GO_BINARY" doctor --json --only=agent-context >"$LAB/go.out" 2>&1 ); GO_STATUS=$?
assert_equals "error" "$(jq -r '.data.checks[0].severity' < "$LAB/go.out")"

test_case "a diagnosis that found something broken fails"
assert_equals "1" "$GO_STATUS"

rm -f "$DIR/AGENTS.md"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "go.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

( cd "$DIR" && "$GO_BINARY" doctor --nonsense >/dev/null 2>&1 ); GO_STATUS=$?
test_case "and an option nobody declared is refused with the usage code"
assert_equals "2" "$GO_STATUS"

# ---------------------------------------------------------------- ported: docker-stop-all
#
# Machine-wide. Non-interactive, the binary stops every running container without asking, so this
# never lets it reach a daemon: DOCKER_HOST points at a socket that does not exist, whatever the
# machine running the suite has up. What is provable that way is that the binary answers the
# command, and answers an unreachable daemon as the docker failure it is — not as "No containers
# running" with exit 0, which read a connection failure as success.
#
# This used to ask `bin/run docker-stop-all`, whose shell twin only exec'd the binary, and to do it
# with the daemon reachable. The twin is retired; the question on the caller's terminal is pinned in
# Go (`internal/cli/docker_stop_all_test.go`, `Interactive`).

test_case "the binary answers docker-stop-all, and with no daemon it reports the docker failure"
answer=$( cd "$DIR" && DOCKER_HOST=unix:///nonexistent HM_NON_INTERACTIVE=1 "$GO_BINARY" --no-json docker-stop-all 2>&1 )
assert_equals "3" "$?"
assert_contains "$answer" "Cannot connect to the Docker daemon"
assert_not_contains "$answer" "No containers running"

# ---------------------------------------------------------------- ported: docker-compose
#
# A Compose subcommand this tool does not implement itself, run exactly the way `docker compose`
# would run it. Read-only — config resolves the files, ps lists what is there — so nothing here
# asks about anything running.
#
# This used to compare the binary with `bin/run docker-compose`, whose shell twin did nothing but
# exec the binary: a comparison of the binary with itself. With the twin retired the oracle is
# the project the lab wrote and `docker compose` itself, neither of them this tool.

go_compose() {
    ( cd "$DIR" && "$GO_BINARY" docker-compose "$@" >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
}

go_compose config --format json
test_case "compose keeps the caller's streams"
assert_equals "0" "$GO_STATUS"
assert_equals "$PROJECT" "$(jq -r '.name' "$LAB/go.out" 2>/dev/null)"
assert_equals "phpfpm" "$(jq -r '.services | keys | join(" ")' "$LAB/go.out" 2>/dev/null)"

go_compose ps
( cd "$DIR" && docker compose -p "$PROJECT" ps >/dev/null 2>&1 ); COMPOSE_STATUS=$?
test_case "and a read-only subcommand that lists something answers with Compose's own status"
assert_equals "$COMPOSE_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- the same refusals
#
# The exit codes are a contract: 2 is a usage error, 4 is not a project, 6 is a refusal on
# purpose. A wrapper that flattened them would break everything that branches on them.

# worktree has no shell twin left to compare with: bin/run would only name the binary, with a 2 of
# its own that matches by coincidence
( cd "$DIR" && "$GO_BINARY" worktree add >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
test_case "a usage error keeps its code"
assert_equals "2" "$GO_STATUS"

( cd "$LAB" && "$GO_BINARY" describe --json >"$LAB/go.out" 2>&1 ); outside=$?
test_case "and so does a directory that is not a project"
assert_equals "4" "$outside"

( cd "$DIR" && "$GO_BINARY" no-existe-este-comando >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?
test_case "an unknown command is a usage error"
assert_equals "2" "$GO_STATUS"
assert_json_field "$(cat "$LAB/go.err")" '.error.type' "command_not_found"

# ---------------------------------------------------------------- what the Go layer resolved
#
# Resolution used to have a diagnostic of its own. It does not need one any more: describe,
# doctor, start and logs all go through the same resolver, so describe's answer is enough.
#
test_case "the Go layer resolves the project from its properties"
resolved=$( cd "$DIR" && "$GO_BINARY" describe --json )
assert_equals "$PROJECT" "$(printf '%s' "$resolved" | jq -r '.data.project.name')"
assert_equals "go.test" "$(printf '%s' "$resolved" | jq -r '.data.project.domain')"
assert_equals "./src" "$(printf '%s' "$resolved" | jq -r '.data.paths.magento_dir')"

test_case "and the same root"
assert_equals "$DIR" "$(printf '%s' "$resolved" | jq -r '.data.project.root')"

#
# The registry. There is no import step to run any more: what earlier versions wrote is read on the
# way in, every time, which is why looking at it is enough to see it.
#
test_case "the registry can be looked at, and brings across what bash wrote"
CASA=$(mktemp -d)
mkdir -p "$CASA/.hm/worktrees/tienda" "$CASA/.hm/state"
printf '{"path":"/code/a","branch":"rama","profile":"agent","domain":"a.test","project":"tienda-a","vendor":"shared"}\n' \
    > "$CASA/.hm/worktrees/tienda/a.json"
printf '{"anonymised_at":"2026-08-02 11:00"}\n' > "$CASA/.hm/state/tienda-a.json"
registro=$( HOME="$CASA" "$GO_BINARY" _registry )
assert_equals "a" "$(printf '%s' "$registro" | jq -r '.data.projects[0].worktrees[0].name')"
assert_equals "shared" "$(printf '%s' "$registro" | jq -r 'if .data.projects[0].worktrees[0].shared_vendor then "shared" else "own" end')"

test_case "and the data of a branch answers for itself, not for its parent"
assert_equals "yes" "$(printf '%s' "$registro" | jq -r '.data.projects[0].worktrees[0].anonymised')"
assert_equals "unknown" "$(printf '%s' "$registro" | jq -r '.data.projects[0].anonymised')"

test_case "importing twice brings nothing twice"
HOME="$CASA" "$GO_BINARY" _registry >/dev/null 2>&1
assert_equals "1" "$( HOME="$CASA" "$GO_BINARY" _registry | jq -r '.data.projects[0].worktrees | length' )"
rm -rf "$CASA"

#
# Which build of the binary is running is a field of `version` now, and the diagnostic that used to
# answer it is gone: it existed while a ported command could not report anything the shell one did
# not, and `version` reports it.
#
test_case "which build the binary is, is part of the version it reports"
assert_contains "$( cd "$DIR" && "$GO_BINARY" --no-json version 2>&1 )" "binary"

# ---------------------------------------------------------------- installed shape
#
# The installation is the checkout with the binary added at bin/hm. It has to find the tool's own
# data beside it, whatever directory it is invoked from and whether it was reached through a
# symlink — which is how /usr/local/bin/hm reaches it. There is no shell tree to find any more.

INSTALL="$LAB/instalado"
mkdir -p "$INSTALL/bin"
cp "$GO_BINARY" "$INSTALL/bin/hm"
cp -R "$COMMAND_BIN_DIR/data" "$INSTALL/data"

test_case "the installed binary answers from anywhere, with no shell tree beside it"
assert_contains "$( cd "$LAB" && "$INSTALL/bin/hm" --no-json --help 2>&1 )" "magento"

test_case "and through the symlink an installation leaves in the path"
ln -sf "$INSTALL/bin/hm" "$LAB/hm-enlazado"
assert_contains "$( cd "$LAB" && "$LAB/hm-enlazado" --no-json --help 2>&1 )" "magento"

test_case "the release builds the binary for both platforms and both architectures"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "goos: [linux]"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "goos: [darwin]"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "goarch: [amd64, arm64]"

# Compose's file watcher reaches FSEvents through cgo, so a darwin binary that is cross-compiled
# with cgo off does not build at all — and the failure is a wall of undefined symbols from a
# package nobody here imports directly
test_case "and asks for cgo on macOS, which is what the watcher needs"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "CGO_ENABLED=1"

test_case "and stamps the version into the binary"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "internal/cli.Version={{.Version}}"

# Both platforms, every time: the tool is used on macOS and on Linux, and a release that quietly
# shipped one of them broken would be found by whoever installed it
test_case "the release is built where both platforms can be built"
assert_contains "$(cat "$COMMAND_BIN_DIR/.github/workflows/release.yml")" "runs-on: macos-latest"
assert_contains "$(cat "$COMMAND_BIN_DIR/.github/workflows/release.yml")" "GOOS=linux"
assert_contains "$(cat "$COMMAND_BIN_DIR/.github/workflows/release.yml")" "GOOS=darwin"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
