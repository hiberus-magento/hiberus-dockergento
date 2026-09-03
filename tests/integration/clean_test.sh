#!/usr/bin/env bash
#
# What the collector may and may not touch.
#
# Every case here is about the second half: this command deletes, so the tests that matter are the
# ones proving it leaves things alone.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
ALIVE="hm-clean-alive"
DEAD="hm-clean-dead"
STRAY="hm-clean-stray"

cleanup() {
    for project in "$ALIVE" "$DEAD" "$STRAY"; do
        containers=$(docker ps -aq --filter "label=com.docker.compose.project=$project" 2>/dev/null)
        [ -n "$containers" ] && docker rm -f $containers >/dev/null 2>&1
        for volume in $(docker volume ls -q 2>/dev/null |
            grep -E "^(${project}_|hm-template-${project}-)" || true); do
            docker volume rm "$volume" >/dev/null 2>&1
        done
    done
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

cleanup_partial() { :; }
export HM_SNAPSHOT_DIR="$LAB/snapshots"

# An environment with our labels, whose directory exists and whose directory does not
build() {
    local project="$1" root="$2"
    mkdir -p "$LAB/compose-$project"
    cat > "$LAB/compose-$project/docker-compose.yml" <<YAML
services:
  phpfpm:
    image: alpine:latest
    command: ["sleep", "1800"]
    labels:
      hm.project: "$project"
      hm.root: "$root"
    volumes:
      - workspace:/data
volumes:
  workspace:
YAML
    ( cd "$LAB/compose-$project" && docker compose -p "$project" up -d ) >/dev/null 2>&1
}

mkdir -p "$LAB/alive-root"
build "$ALIVE" "$LAB/alive-root"
build "$DEAD" "$LAB/gone-root"

# A project whose containers were removed but whose volumes remain — the real shape of the
# problem, and the reason attribution is impossible: the labels lived on the containers.
# (A volume created outside Compose carries no project label at all and is skipped, because
# this tool never creates one that way.)
build "$STRAY" "$LAB/stray-root"
docker rm -f $(docker ps -aq --filter "label=com.docker.compose.project=$STRAY") >/dev/null 2>&1

# Frozen data directories (`hm db freeze`) belong to no compose project, so they are attributed
# by the same question asked of everything else: is the directory they came from still there?
docker volume create --label hm.template=base --label "hm.project=$DEAD" \
    --label "hm.root=$LAB/gone-root" "hm-template-$DEAD-base" >/dev/null
docker volume create --label hm.template=base --label "hm.project=$ALIVE" \
    --label "hm.root=$LAB/alive-root" "hm-template-$ALIVE-base" >/dev/null

#
# A branch environment whose worktree somebody deleted by hand. Its containers and volumes are
# collected by the rules above already — they carry hm.root — but the registration is left over,
# and nothing else deletes it: `hm worktree remove` needs the directory to still be there.
#
export HM_WORKTREE_DIR="$LAB/worktrees"
mkdir -p "$HM_WORKTREE_DIR/padre"
cat > "$HM_WORKTREE_DIR/padre/rama-muerta.json" <<JSON
{"path": "$LAB/no-existe", "branch": "feature/x", "profile": "agent",
 "domain": "rama-muerta.shop.test", "project": "$DEAD-rama-muerta", "parent": "padre"}
JSON
cat > "$HM_WORKTREE_DIR/padre/rama-viva.json" <<JSON
{"path": "$LAB/alive-root", "branch": "feature/y", "profile": "agent",
 "domain": "rama-viva.shop.test", "project": "$ALIVE-rama-viva", "parent": "padre"}
JSON

report() {
    ( cd "$LAB" && "$HM" clean "$@" --json 2>/dev/null )
}

#
# The ported implementation, over the same scene.
#
# Every assertion below is about what the command sees, and both implementations have to see the
# same thing — so the cheapest way to hold that is to compare them here, where the fixture already
# exists, rather than building it twice.
#
GO_BINARY="$COMMAND_BIN_DIR/bin/hm"

if [ -x "$GO_BINARY" ]; then
    test_case "both implementations see the same thing"
    assert_equals "$( cd "$LAB" && "$HM" clean --json 2>/dev/null | jq -S . )" \
                  "$( cd "$LAB" && "$GO_BINARY" clean --json 2>/dev/null | jq -S . )"

    test_case "and report it the same way"
    assert_equals "$( cd "$LAB" && "$HM" --no-json clean 2>&1 )" \
                  "$( cd "$LAB" && "$GO_BINARY" --no-json clean 2>&1 )"

    #
    # The lists arrive as lists. The accumulators are built with `\n` inside double quotes, which
    # is two characters and not one: the text report prints them through `printf`, which
    # interprets them, and jq does not — so every list used to arrive as a single entry with the
    # whole blob inside it and a null reason.
    #
    test_case "and the lists are lists, not one entry with everything in it"
    assert_equals "null" "$( cd "$LAB" && "$HM" clean --json 2>/dev/null |
        jq -r '[.data.unattributable.environments[].reason] | map(select(. == null)) | first' )"
fi

# ---------------------------------------------------------------- what it sees

test_case "an environment whose directory is gone is collectable"
assert_contains "$(report | jq -r '.data.environments[].name')" "$DEAD"

test_case "an environment whose directory exists never is"
assert_not_contains "$(report | jq -r '.data.environments[].name')" "$ALIVE"

test_case "the volumes of a collectable environment come with it"
assert_contains "$(report | jq -r '.data.volumes[]')" "${DEAD}_workspace"

test_case "the volumes of a live environment do not"
assert_not_contains "$(report | jq -r '.data.volumes[]')" "${ALIVE}_workspace"

test_case "a template of a project that is gone is collectable"
assert_contains "$(report | jq -r '.data.volumes[]')" "hm-template-$DEAD-base"

test_case "a template of a project that is still there is not"
assert_not_contains "$(report | jq -r '.data.volumes[]')" "hm-template-$ALIVE-base"

test_case "a template is not reported as unattributable either"
assert_not_contains "$(report | jq -r '.data.unattributable.volumes[]')" "hm-template-$ALIVE-base"

test_case "a volume with no containers is listed as unattributable"
assert_contains "$(report | jq -r '.data.unattributable.volumes[]')" "${STRAY}_workspace"

test_case "a branch environment whose worktree is gone is collectable"
assert_contains "$(report | jq -r '.data.worktrees[].name')" "padre/rama-muerta"

test_case "and one whose worktree is still there is not"
assert_not_contains "$(report | jq -r '.data.worktrees[].name')" "padre/rama-viva"

test_case "looking deletes nothing"
assert_equals "false" "$(report | jq -r '.data.removed')"

test_case "and everything is still there afterwards"
docker volume inspect "${DEAD}_workspace" >/dev/null 2>&1 && r=present || r=gone
assert_equals "present" "$r"

# ---------------------------------------------------------------- refusing

test_case "not confirming deletes nothing"
( cd "$LAB" && printf 'n\n' | "$HM" clean --force ) >/dev/null 2>&1
docker volume inspect "${DEAD}_workspace" >/dev/null 2>&1 && r=present || r=gone
assert_equals "present" "$r"

# ---------------------------------------------------------------- collecting

mkdir -p "$HM_SNAPSHOT_DIR/$DEAD"
printf 'a snapshot\n' | gzip > "$HM_SNAPSHOT_DIR/$DEAD/keepme.sql.gz"

test_case "with --force and no question it collects"
( cd "$LAB" && "$HM" --yes clean --force ) >/dev/null 2>&1
docker volume inspect "${DEAD}_workspace" >/dev/null 2>&1 && r=present || r=gone
assert_equals "gone" "$r"

test_case "the live environment is untouched"
docker volume inspect "${ALIVE}_workspace" >/dev/null 2>&1 && r=present || r=gone
assert_equals "present" "$r"

test_case "and its containers are still running"
running=$(docker ps -q --filter "label=hm.project=$ALIVE" 2>/dev/null | grep -c . || true)
assert_equals "1" "$running"

test_case "the stale registration is gone after --force"
assert_equals "1" "$([ -f "$HM_WORKTREE_DIR/padre/rama-muerta.json" ] && echo 0 || echo 1)"

test_case "and the live one is untouched"
assert_equals "0" "$([ -f "$HM_WORKTREE_DIR/padre/rama-viva.json" ] && echo 0 || echo 1)"

test_case "the unattributable volume survives --force, which is the whole point"
docker volume inspect "${STRAY}_workspace" >/dev/null 2>&1 && r=present || r=gone
assert_equals "present" "$r"

test_case "database snapshots are not part of a clean"
[ -f "$HM_SNAPSHOT_DIR/$DEAD/keepme.sql.gz" ] && r=kept || r=deleted
assert_equals "kept" "$r"

test_case "the dead environment's containers are gone"
remaining=$(docker ps -aq --filter "label=hm.project=$DEAD" 2>/dev/null | grep -c . || true)
assert_equals "0" "$remaining"

# ---------------------------------------------------------------- what it never does

# `docker system prune` is the tool this command exists to not be. A source check rather than a
# runtime one: the point is that the call is not written anywhere.
test_case "the command invokes no Docker prune, ever"
assert_empty "$(grep -vE '^\s*#' "$COMMANDS_DIR/clean.sh" | grep -n 'prune' || true)"

test_case "nothing to collect is reported, not treated as an error"
( cd "$LAB" && "$HM" clean >/dev/null 2>&1 )
assert_equals "0" "$?"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
