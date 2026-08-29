#!/usr/bin/env bash
#
# An environment per branch, resolved by the real router and the real Compose.
#
# Nothing is started here: what is being tested is the resolution — whose project, whose files,
# whose refusals — which is where the damage of WT-01 lived and where this feature has to be
# exactly right.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-worktree-selftest"
BARE="hm-worktree-noproxy"
NETWORK_CREATED=false

cleanup() {
    ( cd "$LAB/$PROJECT" 2>/dev/null && git worktree remove --force "$LAB/$PROJECT-worktrees/feature-x" ) >/dev/null 2>&1
    $NETWORK_CREATED && docker network rm hm-gateway >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

# Registrations go to a throwaway root: a test has no business writing in ~/.hm
export HM_WORKTREE_DIR="$LAB/registry"

if ! docker network inspect hm-gateway >/dev/null 2>&1; then
    docker network create hm-gateway >/dev/null 2>&1 && NETWORK_CREATED=true
fi

make_project() {
    local name="$1" proxy="$2" dir="$LAB/$1"
    mkdir -p "$dir/config/docker" "$dir/app"
    cat > "$dir/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    command: ["sleep", "600"]
    volumes:
      - ./.:/var/www/html
    depends_on:
      - db
      - mailhog
      - rabbitmq
  nginx:
    image: alpine:latest
    volumes:
      - ./app:/var/www/html/app
    ports:
      - 8081:8080
  db:
    image: alpine:latest
  search:
    image: alpine:latest
  redis:
    image: alpine:latest
  varnish:
    image: alpine:latest
    ports:
      - 8080:6081
  hitch:
    image: alpine:latest
  mailhog:
    image: alpine:latest
  rabbitmq:
    image: alpine:latest
YAML
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
    printf '{"MAGENTO_DIR": ".", "DOMAIN": "shop.test", "COMPOSE_PROJECT_NAME": "%s", "USE_PROXY": "%s"}\n' \
        "$name" "$proxy" > "$dir/config/docker/properties.json"

    # A project with dependencies installed, which is the case worth testing
    mkdir -p "$dir/vendor/composer"
    printf '{"packages": []}\n' > "$dir/composer.lock"
    printf '<?php return array();\n' > "$dir/vendor/composer/autoload_psr4.php"

    ( cd "$dir" && git init -q . && git add -A && git -c user.email=t@t -c user.name=t commit -qm init )
}

hm_in() {
    local dir="$1"; shift
    ( cd "$dir" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

make_project "$PROJECT" "true"
make_project "$BARE" "false"

MAIN="$LAB/$PROJECT"
TREE="$LAB/$PROJECT-worktrees/feature-x"

# ---------------------------------------------------------------- refusals first

hm_in "$LAB/$BARE" worktree add feature/x --no-start
assert_equals "6" "$STATUS" "a project not routed through the proxy cannot have branch environments"
assert_contains "$STDERR" "proxy" "and it says why"

hm_in "$MAIN" worktree add feature/x --profile=nonsense --no-start
assert_equals "2" "$STATUS" "an unknown profile is a usage error"
assert_contains "$STDERR" "lite agent full" "it lists the profiles that exist"

# ---------------------------------------------------------------- creating one

hm_in "$MAIN" worktree add feature/x --profile=agent --no-start
assert_equals "0" "$STATUS" "a branch environment is created"

assert_equals "0" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/feature-x.json" ] && echo 0 || echo 1)" \
    "it is registered outside the checkout"

assert_equals "0" "$([ -d "$TREE" ] && echo 0 || echo 1)" \
    "the worktree exists on disk"

assert_equals "" "$( cd "$MAIN" && git status --porcelain )" \
    "and no tracked file of the main checkout changed"

assert_equals "feature/x" "$( cd "$TREE" && git rev-parse --abbrev-ref HEAD )" \
    "the worktree is on the branch"

# ---------------------------------------------------------------- what it resolves to

hm_in "$TREE" docker-compose config --format json
assert_equals "0" "$STATUS" "the branch environment has a valid configuration"

assert_equals "$PROJECT-feature-x" "$(printf '%s' "$STDOUT" | jq -r '.name')" \
    "it is its own compose project"

assert_equals "db nginx phpfpm redis search" \
    "$(printf '%s' "$STDOUT" | jq -r '.services | keys | sort | join(" ")')" \
    "the agent profile is exactly what runs"

assert_equals "5" "$(printf '%s' "$STDOUT" | jq -r '.services | length')" \
    "and nothing else is left in the configuration"

assert_equals "" "$(printf '%s' "$STDOUT" | jq -r '[.services[].ports // empty] | flatten | join(" ")')" \
    "no service publishes a port on the host"

assert_contains "$(printf '%s' "$STDOUT" | jq -r '.services.nginx.labels["traefik.http.routers.'"$PROJECT"'-feature-x.rule"]')" \
    "feature-x.shop.test" "the branch answers on its own address"

#
# The whole point, and the accident WT-01 had to refuse: the mounts resolve here, not in the
# main checkout.
#
assert_equals "$TREE/app" \
    "$(printf '%s' "$STDOUT" | jq -r '.services.nginx.volumes[0].source')" \
    "and its code is the worktree's, not the main checkout's"

# ---------------------------------------------------------------- dependencies
#
# Composer's autoloader resolves its base directory through `__DIR__`, which PHP follows to the
# real path behind a symlink. Linked, the worktree's own modules are never registered and the main
# checkout's code runs. So they are mounted — and the code mount has to survive underneath, which
# is the part that would be catastrophic to get wrong.

test_case "nothing is linked into the worktree"
assert_equals "false" "$([ -L "$TREE/vendor" ] && echo true || echo false)"
assert_equals "false" "$([ -L "$TREE/node_modules" ] && echo true || echo false)"

#
# The mounts are generated on the platform that bind mounts the code, so the overlay is written
# directly here rather than through `worktree add`: what is being checked is that Compose merges
# them the way this depends on, and that is the same on every platform.
#
export COMPONENTS_DIR HELPERS_DIR TASKS_DIR COMMAND_BIN_NAME
source "$TASKS_DIR/worktree_env.sh"

MOUNTS=$(hm_worktree_vendor_mounts "$MAIN" "/var/www/html")
hm_worktree_write_overlay "$LAB/con-vendor.yml" "agent" "feature-x.shop.test" \
    "$PROJECT-feature-x" "phpfpm nginx db search redis varnish hitch mailhog rabbitmq" \
    "hm-gateway" "$MOUNTS"

RESOLVED=$( cd "$TREE" && docker compose \
    -f "$TREE/docker-compose.yml" -f "$LAB/con-vendor.yml" config --format json 2>/dev/null )

test_case "the dependencies of the main checkout are mounted where PHP looks for them"
assert_equals "$MAIN/vendor" \
    "$(printf '%s' "$RESOLVED" | jq -r '.services.phpfpm.volumes[] | select(.target == "/var/www/html/vendor") | .source')"

test_case "read-only, so one branch cannot corrupt what the others read"
assert_equals "true" \
    "$(printf '%s' "$RESOLVED" | jq -r '.services.phpfpm.volumes[] | select(.target == "/var/www/html/vendor") | .read_only')"

test_case "and the code mount is still there underneath"
assert_equals "1" \
    "$(printf '%s' "$RESOLVED" | jq -r '[.services.phpfpm.volumes[] | select(.target == "/var/www/html")] | length')"

test_case "no other service is given the dependencies"
assert_equals "0" \
    "$(printf '%s' "$RESOLVED" | jq -r '[.services.nginx.volumes[]? | select(.target == "/var/www/html/vendor")] | length')"

#
# The refusal reads the recorded decision, so it is tested against a registration that says the
# dependencies are shared — which is what the Linux path writes.
#
test_case "Composer refuses to write to dependencies that are somebody else's"
python3 - "$HM_WORKTREE_DIR/$PROJECT/feature-x.json" <<'INNER'
import json, sys, pathlib
p = pathlib.Path(sys.argv[1])
d = json.loads(p.read_text())
d["vendor"] = "shared"
p.write_text(json.dumps(d))
INNER
hm_in "$TREE" composer install
assert_equals "6" "$STATUS"
assert_contains "$STDERR" "dependencies"

test_case "and says where they belong"
assert_contains "$STDERR" "$MAIN"

test_case "while the rest of Composer is untouched"
hm_in "$TREE" composer --version
assert_equals "0" "$([ "$STATUS" != "6" ] && echo 0 || echo 1)"

python3 - "$HM_WORKTREE_DIR/$PROJECT/feature-x.json" <<'INNER'
import json, sys, pathlib
p = pathlib.Path(sys.argv[1])
d = json.loads(p.read_text())
d["vendor"] = "own"
p.write_text(json.dumps(d))
INNER

# ---------------------------------------------------------------- the guardrails

hm_in "$TREE" stop
assert_equals "0" "$([ "$STATUS" != "6" ] && echo 0 || echo 1)" \
    "lifecycle commands are allowed in a registered worktree"

( cd "$MAIN" && git worktree add -q "$LAB/loose" -b loose ) >/dev/null 2>&1
hm_in "$LAB/loose" stop
assert_equals "6" "$STATUS" "a worktree with no environment of its own is still refused"
assert_contains "$STDERR$STDOUT" "worktree" "naming the main checkout"

hm_in "$TREE" worktree add another/branch --no-start
assert_equals "6" "$STATUS" "branch environments are created from the main checkout"

hm_in "$MAIN" worktree add feature/x --no-start
assert_equals "2" "$STATUS" "the same name is not registered twice"

# ---------------------------------------------------------------- listing

hm_in "$MAIN" worktree list --json
assert_equals "0" "$STATUS" "they can be listed"
assert_equals "feature-x" "$(printf '%s' "$STDOUT" | jq -r '.data.worktrees[0].name')"
assert_equals "agent" "$(printf '%s' "$STDOUT" | jq -r '.data.worktrees[0].profile')"
assert_equals "https://feature-x.shop.test" "$(printf '%s' "$STDOUT" | jq -r '.data.worktrees[0].url')"

# ---------------------------------------------------------------- removing

printf 'uncommitted\n' > "$TREE/scratch.txt"
( cd "$TREE" && git add scratch.txt )
hm_in "$MAIN" worktree remove feature-x
assert_equals "6" "$STATUS" "uncommitted work is not thrown away"

rm -f "$TREE/scratch.txt"
( cd "$TREE" && git reset -q HEAD scratch.txt 2>/dev/null )

hm_in "$MAIN" worktree remove feature-x --force
assert_equals "0" "$STATUS" "a branch environment can be removed"
assert_equals "1" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/feature-x.json" ] && echo 0 || echo 1)" \
    "the registration is gone"
assert_equals "1" "$([ -d "$TREE" ] && echo 0 || echo 1)" \
    "and so is the worktree"

hm_in "$MAIN" worktree remove feature-x --force
assert_equals "2" "$STATUS" "removing what is not there is a usage error"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
