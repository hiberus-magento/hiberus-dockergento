#!/usr/bin/env bash
#
# The lock: taken, given back, broken when its owner is gone, and bounded when it is not.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/print_message.sh"
source "$HELPERS_DIR/exit_codes.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

export HM_LOCK_DIR="$WORK/locks"
export HM_LOCK_TIMEOUT=2
source "$HELPERS_DIR/lock.sh"

# ---------------------------------------------------------------- taking it

test_case "a free lock is taken"
hm_lock_acquire registro && r=tomado || r=no
assert_equals "tomado" "$r"

test_case "and it says who has it"
assert_equals "$$" "$(cat "$HM_LOCK_DIR/registro.lock/pid")"

test_case "a lock already held is not taken twice"
( hm_lock_acquire registro ) && r=tomado || r=no
assert_equals "no" "$r"

test_case "waiting has an end"
before=$(date +%s)
( hm_lock_acquire registro ) >/dev/null 2>&1
assert_equals "true" "$([ $(( $(date +%s) - before )) -le 5 ] && echo true || echo false)"

test_case "giving it back frees it"
hm_lock_release registro
( hm_lock_acquire registro ) && r=tomado || r=no
assert_equals "tomado" "$r"

# ---------------------------------------------------------------- stale locks
#
# An agent killed mid-command would otherwise block every other agent for ever, and whoever finds
# the directory a week later has no way of knowing what it is.

test_case "a lock whose owner is gone is broken, not waited on"
rm -rf "$HM_LOCK_DIR/huerfano.lock"
mkdir -p "$HM_LOCK_DIR/huerfano.lock"
printf '999999\n' > "$HM_LOCK_DIR/huerfano.lock/pid"
hm_lock_acquire huerfano && r=tomado || r=no
assert_equals "tomado" "$r"
hm_lock_release huerfano

test_case "and so is one with no owner at all"
mkdir -p "$HM_LOCK_DIR/sinpid.lock"
hm_lock_acquire sinpid && r=tomado || r=no
assert_equals "tomado" "$r"
hm_lock_release sinpid

test_case "a lock held by a living process is not broken"
sleep 30 &
alive=$!
mkdir -p "$HM_LOCK_DIR/vivo.lock"
printf '%s\n' "$alive" > "$HM_LOCK_DIR/vivo.lock/pid"
hm_lock_is_stale "$HM_LOCK_DIR/vivo.lock" && r=caducado || r=vigente
assert_equals "vigente" "$r"
kill "$alive" 2>/dev/null
wait "$alive" 2>/dev/null

# ---------------------------------------------------------------- running under it

test_case "the work runs with the lock held"
hm_with_lock trabajo bash -c '[ -d "$HM_LOCK_DIR/trabajo.lock" ]' && r=si || r=no
assert_equals "si" "$r"

test_case "and the lock is given back afterwards"
assert_equals "1" "$([ -d "$HM_LOCK_DIR/trabajo.lock" ] && echo 0 || echo 1)"

test_case "a failure inside keeps its exit code"
hm_with_lock trabajo bash -c 'exit 7' && r=0 || r=$?
assert_equals "7" "$r"

test_case "and still gives the lock back"
assert_equals "1" "$([ -d "$HM_LOCK_DIR/trabajo.lock" ] && echo 0 || echo 1)"

# ---------------------------------------------------------------- atomic writes

test_case "a file is written through a temporary and a rename"
printf 'contenido\n' | hm_write_atomically "$WORK/destino.json"
assert_equals "contenido" "$(cat "$WORK/destino.json")"

test_case "and no temporary is left behind"
assert_equals "" "$(ls "$WORK"/destino.json.* 2>/dev/null || true)"

#
# A fixed temporary name is not a temporary file, it is a shared file with a longer name.
#
test_case "two writers at once do not share a temporary"
( printf 'uno\n' | hm_write_atomically "$WORK/carrera.json" ) &
( printf 'dos\n' | hm_write_atomically "$WORK/carrera.json" ) &
wait
assert_equals "true" "$([ "$(cat "$WORK/carrera.json")" = "uno" ] || [ "$(cat "$WORK/carrera.json")" = "dos" ] && echo true || echo false)"

test_case "nothing that failed replaces what was there"
printf 'bueno\n' > "$WORK/existente.json"
false | hm_write_atomically "$WORK/existente.json" 2>/dev/null || true
assert_equals "bueno" "$(cat "$WORK/existente.json")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
