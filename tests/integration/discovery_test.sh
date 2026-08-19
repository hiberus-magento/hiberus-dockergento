#!/usr/bin/env bash
#
# Environment discovery through the hm.* labels.
#
# Creates a throwaway stack labelled like a Dockergento environment, so the labelled path
# is exercised without touching the developer's real projects. Skips itself when Docker is
# not available.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/docker.sh"

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

PROJECT="hm-labels-selftest"
WORKDIR=$(mktemp -d)
GONE_DIR="$WORKDIR/gone"
mkdir -p "$GONE_DIR"

cleanup() {
    docker rm -f $(docker ps -aq --filter "label=hm.project=$PROJECT") >/dev/null 2>&1
    ( cd "$WORKDIR" && docker compose -p "$PROJECT" down --remove-orphans >/dev/null 2>&1 )
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

# An interrupted run leaves containers behind, and their labels would be read as if they
# belonged to this run: start from a clean slate rather than trusting the previous one
docker rm -f $(docker ps -aq --filter "label=hm.project=$PROJECT") >/dev/null 2>&1

cat > "$WORKDIR/docker-compose.yml" <<'YAML'
x-hm-labels: &hm-labels
  hm.project: "${HM_PROJECT:-}"
  hm.root: "${HM_ROOT:-}"
  hm.worktree: "${HM_WORKTREE:-}"
  hm.profile: "${HM_PROFILE:-full}"
  hm.magento: "${HM_MAGENTO:-}"
  hm.version: "${HM_VERSION:-}"
  hm.agent: "${HM_AGENT:-}"

services:
  phpfpm:
    labels: *hm-labels
    image: alpine:latest
    command: ["sleep", "300"]
  db:
    labels: *hm-labels
    image: alpine:latest
    command: ["sleep", "300"]
YAML

( cd "$WORKDIR" && \
  HM_PROJECT="$PROJECT" HM_ROOT="$GONE_DIR" HM_WORKTREE="feature-x" \
  HM_MAGENTO="2.4.9" HM_VERSION="1.5.0" HM_PROFILE="agent" \
  docker compose -p "$PROJECT" up -d >/dev/null 2>&1 )

test_case "the environment shows up in the inventory"
assert_contains "$(hm_environments)" "$PROJECT"

test_case "the environment reports having metadata"
hm_environment_has_metadata "$PROJECT" && r=yes || r=no
assert_equals "yes" "$r"

test_case "the root directory is readable from the labels"
assert_equals "$GONE_DIR" "$(hm_environment_label "$PROJECT" "hm.root")"

test_case "the worktree is readable from the labels"
assert_equals "feature-x" "$(hm_environment_label "$PROJECT" "hm.worktree")"

test_case "the profile is readable from the labels"
assert_equals "agent" "$(hm_environment_label "$PROJECT" "hm.profile")"

test_case "the Magento version is readable from the labels"
assert_equals "2.4.9" "$(hm_environment_label "$PROJECT" "hm.magento")"

test_case "every service of the environment is listed"
assert_equals "2" "$(hm_environment_containers "$PROJECT" | wc -l | tr -d ' ')"

test_case "a service resolves inside its own project"
assert_equals "1" "$(hm_service_container db "$PROJECT" | wc -l | tr -d ' ')"

test_case "a service of another project is never returned"
assert_empty "$(hm_service_container db "$PROJECT-does-not-exist")"

test_case "an existing root directory is not an orphan"
( cd "$WORKDIR" && HM_PROJECT="$PROJECT" HM_ROOT="$WORKDIR" \
  docker compose -p "$PROJECT" up -d --force-recreate >/dev/null 2>&1 )
hm_environment_is_orphan "$PROJECT" && r=orphan || r=alive
assert_equals "alive" "$r"

test_case "a missing root directory marks the environment as orphaned"
( cd "$WORKDIR" && HM_PROJECT="$PROJECT" HM_ROOT="$GONE_DIR" \
  docker compose -p "$PROJECT" up -d --force-recreate >/dev/null 2>&1 )
rmdir "$GONE_DIR"
hm_environment_is_orphan "$PROJECT" && r=orphan || r=alive
assert_equals "orphan" "$r"

test_case "the branch is derived from the root directory, not from a label"
REPO="$WORKDIR/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q -b main
git -C "$REPO" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
( cd "$WORKDIR" && HM_PROJECT="$PROJECT" HM_ROOT="$REPO" \
  docker compose -p "$PROJECT" up -d --force-recreate >/dev/null 2>&1 )
assert_equals "main" "$(hm_environment_branch "$PROJECT")"

test_case "switching branch does not change the labels"
git -C "$REPO" checkout -q -b feature/other
assert_equals "$REPO" "$(hm_environment_label "$PROJECT" "hm.root")"

test_case "switching branch is reflected in the derived branch"
assert_equals "feature/other" "$(hm_environment_branch "$PROJECT")"

# Regression: empty fields used to shift every column, because `read` with IFS=$'\t'
# collapses consecutive tabs. An environment with no worktree and no recorded Magento
# version is exactly the case that exposed it.
source "$TASKS_DIR/collect_environments.sh"

test_case "empty fields do not shift the inventory columns"
( cd "$WORKDIR" && HM_PROJECT="$PROJECT" HM_ROOT="$REPO" HM_WORKTREE="" HM_MAGENTO="" \
  docker compose -p "$PROJECT" up -d --force-recreate >/dev/null 2>&1 )
row=$(collect_environments | jq -c --arg p "$PROJECT" '.[] | select(.name == $p)')
assert_json_field "$row" '.worktree' ""

test_case "the status still lands in its own field"
assert_json_field "$row" '.status' "running"

test_case "the container counts are still numbers"
assert_json_field "$row" '.containers.total' "2"

test_case "the branch is still derived for an environment without metadata fields"
assert_json_field "$row" '.branch' "feature/other"

test_case "containers outside Dockergento are ignored"
assert_not_contains "$(hm_environments)" "definitely-not-a-dockergento-project"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
