#!/usr/bin/env bash
#
# The tool as a library, used from outside.
#
# This is the test the whole separation exists for. `internal/` is a rule of the language, not a
# convention: while the engine lived there no other program could import it, and the web interface
# planned for later would have had nothing to talk to. What proves it is fixed is not that the
# directory moved — it is another module compiling against it and getting real answers.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-go-motor"
trap 'docker compose -p "$PROJECT" down >/dev/null 2>&1; rm -rf "$LAB"; hm_test_home_cleanup' EXIT

if ! command -v go >/dev/null 2>&1; then
    echo "  - skipped: go is not installed"
    echo "RESULT 0 0"
    exit 0
fi

export GOCACHE="$LAB/go-build"

# ---------------------------------------------------------------- a project to ask about

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker" "$DIR/src"
cat > "$DIR/docker-compose.yml" <<'EOF'
services:
  phpfpm:
    image: alpine:latest
    command: sh -c "sleep 900"
    stop_grace_period: 1s
EOF
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "motor.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

# ---------------------------------------------------------------- somebody else's program

CONSUMIDOR="$LAB/consumidor"
mkdir -p "$CONSUMIDOR"

cat > "$CONSUMIDOR/go.mod" <<EOF
module ejemplo/consumidor

go 1.25.0

require github.com/hiberus-magento/hiberus-dockergento v0.0.0

replace github.com/hiberus-magento/hiberus-dockergento => $COMMAND_BIN_DIR
EOF

#
# Deliberately not a hello world: it asks the three questions a web interface would ask on its
# first screen — what is this project, what is running on this machine, and what is wrong.
#
cat > "$CONSUMIDOR/main.go" <<'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
)

func main() {
	motor := dockergento.New(dockergento.Options{Root: os.Args[2]})

	descripcion, err := motor.Describe(os.Args[1], false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	entornos, err := motor.Environments()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	diagnostico, err := motor.Diagnose(os.Args[1], "compose-version")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	salida, _ := json.Marshal(map[string]any{
		"proyecto":     descripcion.Project.Name,
		"dominio":      descripcion.Project.Domain,
		"servicios":    len(descripcion.Services),
		"entornos":     len(entornos),
		"comprobacion": diagnostico.Checks[0].ID,
	})

	fmt.Println(string(salida))
}
EOF

if ! ( cd "$CONSUMIDOR" && GOFLAGS=-mod=mod go build -o consumidor . ) >"$LAB/build.err" 2>&1; then
    test_case "another module can import the engine"
    assert_equals "" "$(cat "$LAB/build.err")"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

test_case "another module can import the engine and build against it"
assert_equals "0" "$?"

# ---------------------------------------------------------------- and get real answers

SALIDA=$( "$CONSUMIDOR/consumidor" "$DIR" "$COMMAND_BIN_DIR" 2>"$LAB/run.err" )
ESTADO=$?

test_case "and it runs"
assert_equals "0" "$ESTADO"

test_case "it resolves the project without being told anything but the directory"
assert_equals "$PROJECT" "$(printf '%s' "$SALIDA" | jq -r '.proyecto')"
assert_equals "motor.test" "$(printf '%s' "$SALIDA" | jq -r '.dominio')"

test_case "it reads the compose configuration"
assert_equals "1" "$(printf '%s' "$SALIDA" | jq -r '.servicios')"

test_case "it can ask Docker what is on this machine"
[ "$(printf '%s' "$SALIDA" | jq -r '.entornos')" -ge 0 ] && r=yes || r=no
assert_equals "yes" "$r"

test_case "and it can run a diagnosis"
assert_equals "compose-version" "$(printf '%s' "$SALIDA" | jq -r '.comprobacion')"

# ---------------------------------------------------------------- and change things

test_case "a consumer that is not the CLI can bring an environment up and stop it"
cat > "$CONSUMIDOR/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
)

func main() {
	motor := dockergento.New(dockergento.Options{Root: os.Args[2]})

	if err := motor.Start(os.Args[1], dockergento.StartOptions{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	descripcion, err := motor.Describe(os.Args[1], false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(descripcion.Project.Status)
}
EOF
( cd "$CONSUMIDOR" && GOFLAGS=-mod=mod go build -o consumidor . ) >/dev/null 2>&1
assert_equals "running" "$( "$CONSUMIDOR/consumidor" "$DIR" "$COMMAND_BIN_DIR" 2>/dev/null | tail -1 )"

# ---------------------------------------------------------------- the CLI stays private

test_case "the command line is not part of what anybody else can import"
mkdir -p "$CONSUMIDOR/privado"
cat > "$CONSUMIDOR/privado/main.go" <<'EOF'
package main

import _ "github.com/hiberus-magento/hiberus-dockergento/internal/cli"

func main() {}
EOF
( cd "$CONSUMIDOR" && GOFLAGS=-mod=mod go build ./privado/ ) >"$LAB/privado.err" 2>&1
assert_contains "$(cat "$LAB/privado.err")" "internal package"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
