#!/usr/bin/env bash
#
# What is installed, where it came from, and whether it is still that.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-aidoctor-selftest"
trap 'rm -rf "$LAB"' EXIT

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "aid.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

run() { ( cd "$DIR" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" ); STATUS=$?; STDOUT=$(cat "$LAB/out"); STDERR=$(cat "$LAB/err"); return 0; }

state_of() {
    printf '%s' "$STDOUT" | jq -r --arg n "$1" '.data.resources[] | select(.name == $n) | .state'
}

# ---------------------------------------------------------------- nothing configured

run ai-doctor --json
test_case "a project with no AI configuration is not an error"
assert_equals "0" "$STATUS"
assert_equals "false" "$(printf '%s' "$STDOUT" | jq -r '.data.configured')"

# ---------------------------------------------------------------- after a pull

cat > "$DIR/config/docker/ai-properties.json" <<'JSON'
{"platforms": ["claude"], "types": [], "resources": ["skills"], "custom_repositories": []}
JSON
export HM_AI_REPOSITORIES="$LAB/repositories.json"
cat > "$HM_AI_REPOSITORIES" <<'JSON'
{"repositories": [{"name": "dockergento", "local": true, "types": ["dockergento"]}]}
JSON

run ai-pull
assert_equals "0" "$STATUS" "the bundled skills are installed"

run ai-doctor --json
test_case "everything just installed is current"
assert_equals "current" "$(state_of dockergento-environment)"
assert_equals "current" "$(state_of dockergento-database)"

test_case "and it records where it came from"
assert_equals "dockergento" "$(printf '%s' "$STDOUT" | jq -r '.data.resources[] | select(.name == "dockergento-agents") | .origin')"

test_case "with the version of the tool that installed it"
assert_equals "true" "$(printf '%s' "$STDOUT" | jq -r '.data.resources[] | select(.name == "dockergento-agents") | .version != "-"')"

#
# The checksum used to be computed with sha256sum, which macOS does not have and which cannot
# digest a directory in any case, so every entry recorded an empty string that looked like a
# checksum. This is the assertion that would have caught it.
#
test_case "the recorded checksum is a real one"
assert_equals "true" \
    "$(jq -r '[.skills[] | select((.checksum // "") | length > 16)] | length > 0' "$DIR/config/docker/ai-registration.json")"

# ---------------------------------------------------------------- a local edit

printf '\nEditado a mano.\n' >> "$DIR/.claude/skills/dockergento-database/SKILL.md"
run ai-doctor --json
test_case "a skill edited by hand is reported as modified"
assert_equals "modified" "$(state_of dockergento-database)"

test_case "and the others are not"
assert_equals "current" "$(state_of dockergento-environment)"

# ---------------------------------------------------------------- something of your own

mkdir -p "$DIR/.claude/skills/mi-skill"
printf 'mía\n' > "$DIR/.claude/skills/mi-skill/SKILL.md"
run ai-doctor --json
test_case "a skill the tool did not install is yours"
assert_equals "custom" "$(state_of mi-skill)"

# ---------------------------------------------------------------- gone

rm -rf "$DIR/.claude/skills/dockergento-debugging"
run ai-doctor --json
test_case "a tracked skill that is gone is reported as missing"
assert_equals "missing" "$(state_of dockergento-debugging)"

# ---------------------------------------------------------------- readable output

run --no-json ai-doctor
test_case "the readable output names the skills and their state"
assert_contains "$STDOUT" "dockergento-environment"
assert_contains "$STDOUT" "custom"

test_case "and warns that hand edits will be lost"
assert_contains "$STDOUT$STDERR" "ai-pull"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
