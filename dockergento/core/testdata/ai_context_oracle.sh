#!/usr/bin/env bash
#
# TEMPORARY SCAFFOLDING: the parity oracle of the Go port of `hm ai-context`.
#
# Delete it together with console/commands/ai-context.sh and console/tasks/agent_context.sh,
# along with the tests that run it (TestAgentContextMatchesTheShellImplementation in
# dockergento/app and TestAIContextPrintsWhatTheShellPrinted in internal/cli).
#
# It runs the shell implementation itself, unmodified, with the one function that needs Docker —
# collect_project_info — replaced by a fixture: the JSON the test hands the Go code as its
# description. Everything else is the real thing: anonymisation.sh reads $HM_STATE_DIR, the block
# comes from agent_context.sh, the exclusions from data/ai-exclusions.json.
#
#   ORACLE_REPO  the checkout whose console/ is run
#   ORACLE_WORK  a scratch directory for the stub (inside the test's temporary directory)
#   ORACLE_INFO  the fixture collect_project_info answers with
#
# HM_ROOT, HM_STATE_DIR, COMPOSE_PROJECT_NAME, COMMAND_BIN_NAME, HM_VERSION and
# HM_OUTPUT_FORMAT are read from the environment, as bin/run would have exported them.
#
set -uo pipefail

: "${ORACLE_REPO:?}" "${ORACLE_WORK:?}" "${ORACLE_INFO:?}"

tasks="$ORACLE_WORK/tasks"
mkdir -p "$tasks"
ln -sf "$ORACLE_REPO/console/tasks/agent_context.sh" "$tasks/agent_context.sh"
ln -sf "$ORACLE_REPO/console/tasks/anonymisation.sh" "$tasks/anonymisation.sh"

cat > "$tasks/collect_project_info.sh" <<'STUB'
collect_project_info() { cat "$ORACLE_INFO"; }
STUB

export COMPONENTS_DIR="$ORACLE_REPO/console/components"
export HELPERS_DIR="$ORACLE_REPO/console/helpers"
export TASKS_DIR="$tasks"
export DATA_DIR="$ORACLE_REPO/data"
export COMMAND_BIN_DIR="$ORACLE_REPO"
export HM_COMMAND="ai-context"

cd "$HM_ROOT" || exit 1
exec bash "$ORACLE_REPO/console/commands/ai-context.sh" "$@"
