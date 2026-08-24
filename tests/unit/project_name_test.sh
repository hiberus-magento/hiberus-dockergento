#!/usr/bin/env bash
#
# The project's identity: how it is derived, and when it is recorded.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/project_name.sh"

# ---------------------------------------------------------------- the rule
#
# This table is not invented: it is what `docker compose config` answers for each of these
# directory names. The integration suite checks it against Compose itself; here it is frozen so
# a change in the rule fails loudly and cheaply.

test_case "lowercased"
assert_equals "upper" "$(hm_derive_project_name "UPPER")"

test_case "spaces are dropped, not replaced"
assert_equals "conespacio" "$(hm_derive_project_name "con espacio")"

test_case "dots are dropped"
assert_equals "puntocom" "$(hm_derive_project_name "punto.com")"

test_case "accented characters are dropped, not transliterated"
assert_equals "acentado" "$(hm_derive_project_name "acentúado")"

test_case "leading dashes and underscores are trimmed"
assert_equals "weird--" "$(hm_derive_project_name "--weird--")"

test_case "dashes and underscores inside are kept"
assert_equals "my_project-1" "$(hm_derive_project_name "my_project-1")"

test_case "a leading digit is fine"
assert_equals "2fast" "$(hm_derive_project_name "2fast")"

test_case "only the last path component counts"
assert_equals "miproyectoraro" "$(hm_derive_project_name "/a/b/Mi Proyecto.Raro")"

test_case "a directory with nothing admissible yields no name"
assert_empty "$(hm_derive_project_name "___")"

test_case "and neither does one made only of symbols"
assert_empty "$(hm_derive_project_name "!!!")"

# ---------------------------------------------------------------- resolution

test_case "a configured name wins and is left untouched"
resolved=$(COMPOSE_PROJECT_NAME="nombre-fijado" HM_ROOT="/tmp/otra-cosa" bash -c '
    source "'"$HELPERS_DIR"'/project_name.sh"
    hm_resolve_project_name
    printf "%s" "$COMPOSE_PROJECT_NAME"')
assert_equals "nombre-fijado" "$resolved"

test_case "without one, the root directory decides"
resolved=$(COMPOSE_PROJECT_NAME="" HM_ROOT="/projects/Tienda Nueva" bash -c '
    source "'"$HELPERS_DIR"'/project_name.sh"
    hm_resolve_project_name
    printf "%s" "$COMPOSE_PROJECT_NAME"')
assert_equals "tiendanueva" "$resolved"

test_case "the root decides, not the working directory"
resolved=$(COMPOSE_PROJECT_NAME="" HM_ROOT="/projects/principal" bash -c '
    cd /tmp
    source "'"$HELPERS_DIR"'/project_name.sh"
    hm_resolve_project_name
    printf "%s" "$COMPOSE_PROJECT_NAME"')
assert_equals "principal" "$resolved"

test_case "resolving assigns, it does not print"
output=$(COMPOSE_PROJECT_NAME="" HM_ROOT="/projects/algo" bash -c '
    source "'"$HELPERS_DIR"'/project_name.sh"
    hm_resolve_project_name')
assert_empty "$output"

test_case "deriving spawns no process"
noise=$(bash -c '
    source "'"$HELPERS_DIR"'/project_name.sh"
    awk() { echo "awk in the derivation"; }
    tr()  { echo "tr in the derivation"; }
    sed() { echo "sed in the derivation"; }
    hm_derive_project_name "Un Directorio.Cualquiera" >/dev/null' 2>&1)
assert_empty "$noise"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
