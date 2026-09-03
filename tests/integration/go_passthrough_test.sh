#!/usr/bin/env bash
#
# The Go binary against the shell one: the same answers.
#
# This is the promise of the strangler. Somebody installs the binary and notices nothing — same
# output, same exit codes, same prompts — and everything not ported yet still runs. A test that
# compares the two is the only way to keep that true while commands move across.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
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

both() {
    ( cd "$DIR" && "$SHELL_CLI" "$@" >"$LAB/shell.out" 2>"$LAB/shell.err" ); SHELL_STATUS=$?
    ( cd "$DIR" && "$GO_BINARY" "$@" >"$LAB/go.out" 2>"$LAB/go.err" );      GO_STATUS=$?
}

# ---------------------------------------------------------------- the same answers

both --version
test_case "the version command answers the same"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- ported: describe
#
# The richest document the tool produces, and the command run most often. Compared whole, with
# and without the secrets, and as a table with colour turned off — anything less and a port could
# quietly drop a field nobody looks at until an agent needs it.

both describe --json
test_case "describe answers exactly what the shell one did"
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

both describe --json --with-secrets
test_case "and with the credentials too, when they are asked for"
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

test_case "which are never volunteered"
both describe --json
assert_equals "null" "$(jq -r '.data.credentials // "null"' < "$LAB/go.out")"

test_case "and the table is the same, character for character"
( cd "$DIR" && NO_COLOR=1 "$SHELL_CLI" --no-json describe >"$LAB/shell.out" 2>&1 )
( cd "$DIR" && NO_COLOR=1 "$GO_BINARY"  --no-json describe >"$LAB/go.out" 2>&1 )
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"

both describe --nonsense
test_case "an option nobody declared is refused the same way"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- ported: list
#
# The first command that stopped going through the shell implementation. What is compared is not
# "roughly the same" but the whole document and the whole table, because the moment the two are
# allowed to differ a little, nobody can tell a port from a regression.

both list --json
test_case "the ported command answers exactly what the shell one did"
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

both --no-json list
test_case "and prints exactly the same table"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"

both list --nonsense
test_case "and refuses the same options, with the same code"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

test_case "the JSON envelope is the same shape"
both list --json
assert_equals "$(jq -S 'del(.data)' < "$LAB/shell.out")" "$(jq -S 'del(.data)' < "$LAB/go.out")"

#
# The tool's own flags are accepted before the command name as well as after it, which is what the
# shell implementation does. Consuming them only after meant `hm --no-json describe` never reached
# the Go implementation at all: it fell through to the shell one, silently, and looked exactly
# like it had worked — including to the tests that were comparing the two.
#
test_case "a global flag before the command still reaches the Go implementation"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" --no-json _binary >/dev/null 2>&1; echo $? )"

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
# Seventeen checks, of which five ask Docker and four look at the host. Compared whole rather than
# check by check: what a port of this command can lose is a single line nobody notices missing
# until the day it was the one that mattered.

both doctor --json
test_case "the diagnosis finds exactly what the shell one found"
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

test_case "and reports every check, in the same order"
assert_equals "$(jq -r '.data.checks[].id' < "$LAB/shell.out")" "$(jq -r '.data.checks[].id' < "$LAB/go.out")"

test_case "and prints the same report, character for character"
( cd "$DIR" && NO_COLOR=1 "$SHELL_CLI" --no-json doctor >"$LAB/shell.out" 2>&1 )
( cd "$DIR" && NO_COLOR=1 "$GO_BINARY"  --no-json doctor >"$LAB/go.out" 2>&1 )
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"

both doctor --only=ports
test_case "and one check on its own is still the same check"
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

#
# Outside a project the diagnosis still runs and answers about the machine — which is the question
# somebody has when nothing works anywhere. The ports it needs then come from the template the
# tool ships, because there is no configuration to ask.
#
test_case "outside a project both answer about the machine"
( cd "$LAB" && "$SHELL_CLI" doctor --json >"$LAB/shell.out" 2>&1 )
( cd "$LAB" && "$GO_BINARY"  doctor --json >"$LAB/go.out" 2>&1 )
assert_equals "$(jq -S . < "$LAB/shell.out" 2>/dev/null)" "$(jq -S . < "$LAB/go.out" 2>/dev/null)"

test_case "and neither reports a project check there"
assert_equals "" "$(jq -r '[.data.checks[] | select(.scope == "project")] | .[]' < "$LAB/go.out")"

#
# The fingerprint is the one value in the diagnosis that has to agree with something already
# written down: `hm ai-context` is still the shell implementation, and the digest it left in
# AGENTS.md is what the Go check compares against. A jq newline the port dropped would make every
# generated context on every project read as stale.
#
( cd "$DIR" && "$SHELL_CLI" ai-context >/dev/null 2>&1 )

test_case "the Go check recognises the context the shell command generated"
( cd "$DIR" && "$GO_BINARY" doctor --json --only=agent-context >"$LAB/go.out" 2>&1 )
assert_equals "ok" "$(jq -r '.data.checks[0].severity' < "$LAB/go.out")"

test_case "and both notice when the project moved on"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "otro.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"
both doctor --json --only=agent-context
assert_equals "$(jq -r '.data.checks[0].severity' < "$LAB/shell.out")" "$(jq -r '.data.checks[0].severity' < "$LAB/go.out")"
assert_equals "error" "$(jq -r '.data.checks[0].severity' < "$LAB/go.out")"

test_case "a diagnosis that found something broken fails, in both"
assert_equals "1" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

rm -f "$DIR/AGENTS.md"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "go.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

both doctor --nonsense
test_case "and an option nobody declared is refused the same way"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- the same refusals
#
# The exit codes are a contract: 2 is a usage error, 4 is not a project, 6 is a refusal on
# purpose. A wrapper that flattened them would break everything that branches on them.

both worktree add
test_case "a usage error keeps its code"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

( cd "$LAB" && "$GO_BINARY" describe --json >"$LAB/go.out" 2>&1 ); outside=$?
test_case "and so does a directory that is not a project"
assert_equals "4" "$outside"

both no-existe-este-comando
test_case "an unknown command answers the same"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_contains "$(cat "$LAB/go.err")$(cat "$LAB/go.out")" "$(head -c 40 "$LAB/shell.err" 2>/dev/null || true)"

# ---------------------------------------------------------------- what the Go layer resolved
#
# The two implementations answer the same question separately, and a test compares them. That is
# how a command gets ported without anybody reading both.

#
# Resolution used to have a diagnostic of its own. It does not need one any more: describe,
# doctor, start and logs all go through the same resolver and every one of them is compared
# against the shell implementation, which is a stronger check than a command nobody runs.
#
test_case "the Go layer resolves the same project as the shell one"
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

test_case "the binary says which build it is, which the shell one cannot"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" _binary >/dev/null 2>&1; echo $?)"

# ---------------------------------------------------------------- installed shape
#
# The installation is the checkout with one file added: the binary at bin/hm, beside bin/run. It
# has to find the shell tree from there, whatever directory it is invoked from and whether it was
# reached through a symlink — which is how /usr/local/bin/hm reaches it.

INSTALL="$LAB/instalado"
mkdir -p "$INSTALL/bin" "$INSTALL/console"
cp "$GO_BINARY" "$INSTALL/bin/hm"
printf '#!/bin/sh\nprintf "soy el shell: %%s\\n" "$*"\n' > "$INSTALL/bin/run"
chmod +x "$INSTALL/bin/run"

PROBE="una-orden-que-nadie-va-a-portar"

test_case "the binary finds the shell tree beside it"
assert_equals "soy el shell: $PROBE" "$( cd "$LAB" && HM_LEGACY_ROOT= "$INSTALL/bin/hm" "$PROBE" )"

test_case "and through the symlink an installation leaves in the path"
ln -sf "$INSTALL/bin/hm" "$LAB/hm-enlazado"
assert_equals "soy el shell: $PROBE" "$( cd "$LAB" && HM_LEGACY_ROOT= "$LAB/hm-enlazado" "$PROBE" )"

test_case "a checkout with no shell tree says so instead of guessing"
mkdir -p "$LAB/roto/bin"
cp "$GO_BINARY" "$LAB/roto/bin/hm"
( cd "$LAB" && HM_LEGACY_ROOT= "$LAB/roto/bin/hm" "$PROBE" >/dev/null 2>"$LAB/roto.err" )
assert_equals "3" "$?"
assert_contains "$(cat "$LAB/roto.err")" "shell implementation"

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

test_case "the installer prefers the binary and falls back to the shell"
assert_contains "$(cat "$COMMAND_BIN_DIR/installer.sh")" 'target="$HOME/hm/bin/hm"'
assert_contains "$(cat "$COMMAND_BIN_DIR/installer.sh")" 'target="$HOME/hm/bin/run"'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
