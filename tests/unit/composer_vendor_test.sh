#!/usr/bin/env bash
#
# The macOS vendor dance in composer.sh, up to its first step: restarting phpfpm, which goes
# through the binary (`hm_run_binary restart`) and so through the binary's worktree guard.
#
# From a git worktree with no environment of its own the binary refuses with 6. composer.sh runs
# under set -e, so that refusal has to stop it right there, before the dependencies are copied in
# or anything is run in a container that belongs to the main checkout. Nothing real is reached:
# the binary and the shell commands composer.sh calls are stand-ins that record what they were
# asked, and no Docker is involved.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB="$HM_TEST_HOME/composer-vendor"
CALLS="$LAB/calls"
mkdir -p "$LAB/bin" "$LAB/commands" "$LAB/magento"

# The binary refuses restart the way it does from a worktree with no environment of its own, and
# accepts anything else
cat > "$LAB/bin/hm" <<FAKE
#!/usr/bin/env bash
printf 'hm %s\n' "\$*" >> "$CALLS"
[ "\${1:-}" == "restart" ] && exit 6
exit 0
FAKE

# Whatever composer.sh would run next, recorded so its absence can be asserted
for name in exec stop start; do
    printf '#!/usr/bin/env bash\nprintf "%s.sh %%s\\n" "$*" >> "%s"\n' "$name" "$CALLS" \
        > "$LAB/commands/$name.sh"
done
chmod +x "$LAB/bin/hm" "$LAB/commands/"*.sh

run_composer() {
    : > "$CALLS"
    COMMAND_BIN_DIR="$LAB" COMMANDS_DIR="$LAB/commands" MACHINE=mac MAGENTO_DIR="$LAB/magento" \
        HM_WORKTREE_ENVIRONMENT=false DOCKER_COMPOSE="false" \
        bash "$HM_TEST_PROJECT_ROOT/console/commands/composer.sh" "$@" >/dev/null 2>&1
    STATUS=$?
}

test_case "composer install on macOS stops with the binary's refusal of the restart"
run_composer install
assert_equals "6" "$STATUS"

test_case "and the restart was asked of the binary, for phpfpm"
assert_equals "hm restart phpfpm" "$(head -n 1 "$CALLS")"

test_case "and nothing ran after it: no copy into the container, no composer"
assert_equals "1" "$(grep -c . "$CALLS")"

test_case "a subcommand that does not write the dependencies never restarts anything"
run_composer show
assert_equals "exec.sh composer show" "$(cat "$CALLS")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
