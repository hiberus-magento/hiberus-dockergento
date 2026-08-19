#!/usr/bin/env bash
#
# Values exported for the hm.* labels.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/set_environment_labels.sh"

# Note: variables are exported inside the subshell instead of using the
# `VAR=value function` prefix form, whose scope after the call differs between
# bash 3.2 (macOS) and bash 5 (Linux).

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

test_case "the Magento version is read from composer.lock"
cat > "$WORKDIR/composer.lock" <<'JSON'
{"packages":[{"name":"vendor/other","version":"1.0.0"},
             {"name":"magento/product-community-edition","version":"2.4.9"}]}
JSON
assert_equals "2.4.9" "$(MAGENTO_DIR="$WORKDIR" hm_magento_version)"

test_case "the enterprise edition is recognised too"
cat > "$WORKDIR/composer.lock" <<'JSON'
{"packages":[{"name":"magento/product-enterprise-edition","version":"2.4.8-p1"}]}
JSON
assert_equals "2.4.8-p1" "$(MAGENTO_DIR="$WORKDIR" hm_magento_version)"

test_case "a project without composer.lock reports nothing"
rm -f "$WORKDIR/composer.lock"
assert_empty "$(MAGENTO_DIR="$WORKDIR" hm_magento_version)"

test_case "a malformed composer.lock does not break the command"
echo "not json at all" > "$WORKDIR/composer.lock"
assert_empty "$(MAGENTO_DIR="$WORKDIR" hm_magento_version)"

test_case "a composer.lock without Magento reports nothing"
echo '{"packages":[{"name":"vendor/other","version":"1.0.0"}]}' > "$WORKDIR/composer.lock"
assert_empty "$(MAGENTO_DIR="$WORKDIR" hm_magento_version)"

test_case "the project name comes from the compose project"
( export COMPOSE_PROJECT_NAME=demo MAGENTO_DIR="$WORKDIR"
  set_environment_labels
  echo "$HM_PROJECT" ) > "$WORKDIR/out"
assert_equals "demo" "$(cat "$WORKDIR/out")"

test_case "the root defaults to the current directory"
( cd "$WORKDIR" || exit 1
  export COMPOSE_PROJECT_NAME=demo MAGENTO_DIR="." HM_ROOT=""
  set_environment_labels
  echo "$HM_ROOT" ) > "$WORKDIR/out"
assert_equals "$WORKDIR" "$(cat "$WORKDIR/out")"

test_case "the profile defaults to full"
( export COMPOSE_PROJECT_NAME=demo MAGENTO_DIR="$WORKDIR" HM_PROFILE=""
  set_environment_labels
  echo "$HM_PROFILE" ) > "$WORKDIR/out"
assert_equals "full" "$(cat "$WORKDIR/out")"

test_case "the agent is taken from the environment when present"
( export COMPOSE_PROJECT_NAME=demo MAGENTO_DIR="$WORKDIR" HM_AGENT="claude-3"
  set_environment_labels
  echo "$HM_AGENT" ) > "$WORKDIR/out"
assert_equals "claude-3" "$(cat "$WORKDIR/out")"

test_case "the worktree is empty outside a worktree"
( export COMPOSE_PROJECT_NAME=demo MAGENTO_DIR="$WORKDIR" HM_WORKTREE=""
  set_environment_labels
  echo "[$HM_WORKTREE]" ) > "$WORKDIR/out"
assert_equals "[]" "$(cat "$WORKDIR/out")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
