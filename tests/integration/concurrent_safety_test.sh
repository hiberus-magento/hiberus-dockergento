#!/usr/bin/env bash
#
# What happens when two of these run at once.
#
# The tool was written for one person doing one thing at a time. These are the places where that
# assumption shows, checked by actually doing two things at once rather than by reading the code.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-concurrent-selftest"
trap 'rm -rf "$LAB"' EXIT

export HM_LOCK_DIR="$LAB/locks"
export HM_WORKTREE_DIR="$LAB/worktrees"
export HM_STATE_DIR="$LAB/state"

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker" "$DIR/app/etc"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    volumes:
      - ./.:/var/www/html
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "principal.test", "COMPOSE_PROJECT_NAME": "%s", "USE_PROXY": "true"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"
( cd "$DIR" && git init -q . && git add -A && git -c user.email=t@t -c user.name=t commit -qm init ) >/dev/null 2>&1

# ---------------------------------------------------------------- a worktree's own properties
#
# The properties directory was derived from the project root before the tool knew it was in a
# worktree, so a worktree read — and `save_properties` wrote — the main checkout's.

( cd "$DIR" && git worktree add -q "$LAB/rama" -b rama ) >/dev/null 2>&1
mkdir -p "$HM_WORKTREE_DIR/$PROJECT"
# A registered worktree carries an overlay as well as a registration; `worktree add` writes both
printf 'services:\n  phpfpm:\n    ports: !reset []\n' > "$HM_WORKTREE_DIR/$PROJECT/rama.yml"

cat > "$HM_WORKTREE_DIR/$PROJECT/rama.json" <<JSON
{"path": "$LAB/rama", "branch": "rama", "profile": "agent", "domain": "rama.principal.test",
 "project": "$PROJECT-rama", "parent": "$PROJECT", "vendor": "own", "created": "2026-01-01 00:00"}
JSON

# The branch changes a property that `describe` reports back, which is what tells the two files
# apart from the outside
python3 - "$LAB/rama/config/docker/properties.json" <<'INNER'
import json, sys, pathlib
p = pathlib.Path(sys.argv[1])
d = json.loads(p.read_text())
d["MAGENTO_DIR"] = "./src"
p.write_text(json.dumps(d))
INNER

test_case "a worktree reads its own properties, not the main checkout's"
assert_equals "./src" "$( cd "$LAB/rama" && "$HM" describe --json 2>/dev/null | jq -r '.data.paths.magento_dir')"

test_case "and the main checkout still reads its own"
assert_equals "." "$( cd "$DIR" && "$HM" describe --json 2>/dev/null | jq -r '.data.paths.magento_dir')"

test_case "which is the file that was there all along"
assert_equals "." "$(jq -r '.MAGENTO_DIR' "$DIR/config/docker/properties.json")"

# ---------------------------------------------------------------- two at once
#
# Two registrations written at the same time used to be a plain redirection each: one of the two
# files could be left half written, and a half-written JSON is a worktree nobody can list again.

test_case "two registrations at once leave two whole files"
(
    source "$COMPONENTS_DIR/print_message.sh"
    source "$HELPERS_DIR/exit_codes.sh"
    source "$TASKS_DIR/worktree_env.sh"
    for i in 1 2 3 4 5 6 7 8; do
        ( hm_worktree_save "$PROJECT" "carrera-$i" "/code/$i" "rama-$i" agent "d$i.test" "p-$i" own ) &
    done
    wait
) >/dev/null 2>&1

whole=0
for i in 1 2 3 4 5 6 7 8; do
    jq -e . "$HM_WORKTREE_DIR/$PROJECT/carrera-$i.json" >/dev/null 2>&1 && whole=$((whole + 1))
done
assert_equals "8" "$whole"

test_case "and no temporary files behind them"
assert_equals "" "$(ls "$HM_WORKTREE_DIR/$PROJECT"/*.XXXXXX* 2>/dev/null || true)"
assert_equals "8" "$(ls "$HM_WORKTREE_DIR/$PROJECT"/carrera-*.json | wc -l | tr -d ' ')"

# ---------------------------------------------------------------- collisions

test_case "a branch whose name is already taken by another branch is refused"
( cd "$DIR" && "$HM" worktree add Rama --no-start >"$LAB/out" 2>"$LAB/err" )
status=$?
assert_equals "2" "$status"
assert_contains "$(cat "$LAB/err")" "belongs to"

# ---------------------------------------------------------------- hosts entries

test_case "the entries this tool adds carry a marker"
assert_contains "$(cat "$COMMANDS_DIR/set-host.sh")" 'added by $COMMAND_BIN_NAME'

test_case "and there is a way to remove its own"
assert_contains "$(cat "$COMMANDS_DIR/set-host.sh")" "remove_local_host"

test_case "the collection command reports them without touching the file"
before=$(md5 -q /etc/hosts 2>/dev/null || md5sum /etc/hosts | cut -d' ' -f1)
( cd "$LAB" && "$HM" clean --json >/dev/null 2>&1 )
after=$(md5 -q /etc/hosts 2>/dev/null || md5sum /etc/hosts | cut -d' ' -f1)
assert_equals "$before" "$after"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
