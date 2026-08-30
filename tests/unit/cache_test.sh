#!/usr/bin/env bash
#
# What proves a cached value is still good.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

export HM_CACHE_DIR="$WORK/cache"
source "$HELPERS_DIR/cache.sh"

FILE="$WORK/docker-compose.yml"
printf 'services:\n  phpfpm:\n    image: alpine\n' > "$FILE"

# ---------------------------------------------------------------- the token
#
# It used to be the modification time alone, which has a granularity of one second: a file
# rewritten within the same second as the last check went unnoticed. That is not a rare
# alignment — it is what happens when a command runs, something edits the file, and the next
# command runs straight after.

test_case "the same file gives the same token"
assert_equals "$(hm_file_token "$FILE")" "$(hm_file_token "$FILE")"

test_case "a file rewritten in the same second with different content gives a different token"
before=$(hm_file_token "$FILE")
printf 'services:\n  phpfpm:\n    image: alpine\n  nginx:\n    image: alpine\n' > "$FILE"
assert_equals "distinto" \
    "$([ "$before" != "$(hm_file_token "$FILE")" ] && echo distinto || echo igual)"

#
# On Alpine, busybox's `stat -f` means "filesystem status": it printed a block of filesystem
# information and exited successfully, so the fallback never ran and the token was a multi-line
# blob that no cache could compare. Nothing noticed, because a cache that always misses looks
# like a cache that works.
#
test_case "the token is one line, whichever stat this machine has"
assert_equals "1" "$(hm_file_token "$FILE" | grep -c . )"

test_case "and it is a time and a size, not a paragraph"
assert_equals "true" "$(hm_file_token "$FILE" | grep -qE '^[0-9]+-[0-9]+$' && echo true || echo false)"

test_case "the modification time is one line too"
assert_equals "1" "$(hm_file_mtime "$FILE" | grep -c . )"

test_case "a file that is not there has a token all the same"
assert_equals "true" "$([ -n "$(hm_file_token "$WORK/no-existe")" ] && echo true || echo false)"

# ---------------------------------------------------------------- reading and writing

test_case "a value written is read back"
hm_cache_write clave "$(hm_file_token "$FILE")" valido
assert_equals "valido" "$(hm_cache_read clave "$(hm_file_token "$FILE")")"

test_case "and a stale token is a miss, not an error"
hm_cache_read clave "otro-token" >/dev/null 2>&1 && r=acierto || r=fallo
assert_equals "fallo" "$r"

test_case "an entry that is not there is a miss"
hm_cache_read nunca-escrita cualquiera >/dev/null 2>&1 && r=acierto || r=fallo
assert_equals "fallo" "$r"

test_case "a corrupted entry is treated as absent"
printf 'solo-una-linea\n' > "$HM_CACHE_DIR/rota"
hm_cache_read rota solo-una-linea >/dev/null 2>&1 && r=acierto || r=fallo
assert_equals "fallo" "$r"

test_case "the cache lives outside the project, where it cannot be committed"
assert_contains "$(cat "$HELPERS_DIR/cache.sh")" 'HM_CACHE_DIR="${HM_CACHE_DIR:-$HOME/.hm/cache}"'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
