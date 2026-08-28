#!/usr/bin/env bash
#
# The skills that come with the tool are installed from the tool, not downloaded.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-skills-selftest"
trap 'rm -rf "$LAB"' EXIT

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "skills.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"

# The AI configuration a person would have after `hm ai-init`
cat > "$DIR/config/docker/ai-properties.json" <<'JSON'
{
  "platforms": ["claude"],
  "types": ["hyva"],
  "resources": ["skills"],
  "custom_repositories": []
}
JSON

#
# Only the local repository: the point of the test is that nothing is downloaded, and a suite
# that reaches GitHub fails on a train
#
export HM_AI_REPOSITORIES="$LAB/repositories.json"
cat > "$HM_AI_REPOSITORIES" <<'JSON'
{
  "repositories": [
    {
      "name": "dockergento",
      "local": true,
      "types": ["dockergento"],
      "description": "The skills that come with this tool"
    }
  ]
}
JSON

( cd "$DIR" && "$HM" ai-pull >"$LAB/out" 2>"$LAB/err" )
STATUS=$?

test_case "pulling succeeds with no network"
assert_equals "0" "$STATUS"

test_case "the bundled skills are installed"
for skill in environment database debugging agents; do
    assert_equals "0" "$([ -f "$DIR/.claude/skills/dockergento-$skill/SKILL.md" ] && echo 0 || echo 1)"
done

test_case "they are the ones this copy of the tool carries"
assert_equals "$(cat "$COMMAND_BIN_DIR/skills/dockergento-environment/SKILL.md")" \
    "$(cat "$DIR/.claude/skills/dockergento-environment/SKILL.md")"

test_case "and they are installed even though the configured types do not name them"
assert_contains "$(cat "$DIR/config/docker/ai-properties.json")" '"hyva"'

test_case "the installation is tracked, so a later reset knows what it may remove"
assert_contains "$(cat "$DIR/config/docker/ai-registration.json" 2>/dev/null)" "dockergento-environment"

#
# A skill somebody wrote themselves is not overwritten by an update. It is the property that
# makes it safe to run this on a repository that has its own.
#
mkdir -p "$DIR/.claude/skills/mi-skill-propia"
printf 'mía\n' > "$DIR/.claude/skills/mi-skill-propia/SKILL.md"
( cd "$DIR" && "$HM" ai-pull >/dev/null 2>&1 )

test_case "a custom skill survives an update"
assert_equals "mía" "$(cat "$DIR/.claude/skills/mi-skill-propia/SKILL.md")"

test_case "and pulling twice is not an error"
( cd "$DIR" && "$HM" ai-pull >/dev/null 2>&1 )
assert_equals "0" "$?"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
