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
# Piped output is being read by a program, and a program reading a table of dashes breaks the
# first time a column widens. Both implementations answer JSON when nobody is watching.
#
test_case "both default to JSON when the output is not a terminal"
assert_equals "1" "$( cd "$DIR" && "$GO_BINARY" list | jq -r '.schema_version' )"

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

test_case "the Go layer resolves the same project as the shell one"
resolved=$( cd "$DIR" && "$GO_BINARY" hm-go-project )
assert_equals "$PROJECT" "$(printf '%s' "$resolved" | jq -r '.name')"
assert_equals "go.test" "$(printf '%s' "$resolved" | jq -r '.domain')"
assert_equals "./src" "$(printf '%s' "$resolved" | jq -r '.magento_dir')"

test_case "and the same root"
assert_equals "$DIR" "$(printf '%s' "$resolved" | jq -r '.root')"

test_case "a project that has not chosen a topology is classic"
assert_equals "classic" "$(printf '%s' "$resolved" | jq -r '.topology')"

test_case "the binary says which build it is, which the shell one cannot"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" hm-go-version >/dev/null 2>&1; echo $?)"

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
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "goos: [darwin, linux]"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "goarch: [amd64, arm64]"

test_case "and stamps the version into the binary"
assert_contains "$(cat "$COMMAND_BIN_DIR/.goreleaser.yaml")" "internal/cli.Version={{.Version}}"

test_case "the installer prefers the binary and falls back to the shell"
assert_contains "$(cat "$COMMAND_BIN_DIR/installer.sh")" 'target="$HOME/hm/bin/hm"'
assert_contains "$(cat "$COMMAND_BIN_DIR/installer.sh")" 'target="$HOME/hm/bin/run"'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
