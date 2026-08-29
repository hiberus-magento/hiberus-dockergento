#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$COMPONENTS_DIR"/progress.sh
source "$HELPERS_DIR"/docker.sh

#
# Does what has been written hold up?
#
# With agents writing code the bottleneck stops being writing and becomes checking, and a check
# that depends on remembering four tools with four syntaxes does not happen.
#
# The tools are not the same in any two projects — of the fourteen on the machine this was written
# for, ten had PHPUnit, six PHPStan, five the Magento standard and three had nothing at all. So a
# fixed list of checks would fail almost everywhere: this looks at what is there and runs that.
# Missing is reported as skipped, never as failure: "this project has no PHPStan" and "PHPStan
# failed" lead to opposite actions.
#

run_everything=false
changed_only=false

for argument in "$@"; do
    case "$argument" in
        --all)     run_everything=true ;;
        --changed) changed_only=true ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $argument" \
                "$COMMAND_BIN_NAME verify [--changed] [--all]"
            ;;
    esac
done

is_run_service "phpfpm"

in_container() {
    $DOCKER_COMPOSE exec -T phpfpm bash -c "$1" 2>&1
}

has_binary() {
    $DOCKER_COMPOSE exec -T phpfpm test -x "vendor/bin/$1" >/dev/null 2>&1
}

#
# Which files to look at.
#
# `--changed` compares against where this branch left the main one, not against the last commit:
# what matters when closing a task is everything the branch changed. Without a base to compare
# against, everything is checked and that is said, rather than checking a part while believing it
# is the whole.
#
FILES=""
SCOPE="everything"

if $changed_only; then
    base=""
    for candidate in main master; do
        git -C "$HM_ROOT" rev-parse --verify "$candidate" >/dev/null 2>&1 && { base="$candidate"; break; }
    done

    if [ -n "$base" ]; then
        FILES=$(git -C "$HM_ROOT" diff --name-only --diff-filter=ACMR "$base"...HEAD -- '*.php' 2>/dev/null)
        SCOPE="changed"
    else
        SCOPE="everything (no base branch to compare against)"
    fi
fi

RESULTS=""

#
# One record per line, fields separated by tabs — so a tool's output, which is many lines, cannot
# be stored raw: it would split one record into several and the whole report becomes nonsense.
# Newlines are folded into a record separator and unfolded when rendering.
#
record() {
    local output
    output=$(printf '%s' "${4:-}" | tr '\n' '\036')
    RESULTS="${RESULTS}${1}\t${2}\t${3}\t${output}\n"
}

unfold() {
    printf '%s' "$1" | tr '\036' '\n'
}

# ------------------------------------------------------------------ syntax

#
# The one check that needs nothing installed, and the one that catches what hurts most: a file that
# takes the whole site down. In a project with no tooling at all, `hm verify` still does something.
#
check_syntax() {
    local target="app/code app/design"
    local output

    hm_start "Checking syntax..."

    if [ "$SCOPE" == "changed" ]; then
        [ -z "$FILES" ] && { hm_stop 0 "nothing to check"; record "syntax" "skipped" "0" "no PHP files changed"; return 0; }
        output=$(in_container "for f in $(printf '%s' "$FILES" | tr '\n' ' '); do [ -f \"\$f\" ] && php -l \"\$f\"; done | grep -v '^No syntax errors' || true")
    else
        output=$(in_container "find $target -name '*.php' -print0 2>/dev/null | xargs -0 -n1 -P4 php -l 2>&1 | grep -v '^No syntax errors' || true")
    fi

    local problems
    problems=$(printf '%s' "$output" | sed '/^$/d' | grep -c . || true)

    hm_stop 0

    if [ "${problems:-0}" -eq 0 ]; then
        record "syntax" "ok" "0" ""
    else
        record "syntax" "failed" "$problems" "$output"
    fi
}

# ------------------------------------------------------------------ the optional ones

check_with_binary() {
    local name="$1" binary="$2" command="$3" reason="$4"

    if ! has_binary "$binary"; then
        record "$name" "skipped" "0" "$reason"
        return 0
    fi


    #
    # Each of these is a minute of a PHP process saying nothing. The label goes up before the
    # process starts, which is the whole rule.
    #
    hm_start "Running $binary..."

    local output status
    output=$(in_container "$command")
    status=$?

    hm_stop 0

    if [ "$status" -eq 0 ]; then
        record "$name" "ok" "0" ""
    else
        local problems
        problems=$(printf '%s' "$output" | sed '/^$/d' | grep -c . || true)
        record "$name" "failed" "${problems:-1}" "$output"
    fi
}

files_argument() {
    if [ "$SCOPE" == "changed" ] && [ -n "$FILES" ]; then
        printf '%s' "$FILES" | tr '\n' ' '
    else
        printf 'app/code'
    fi
}

# ------------------------------------------------------------------ run them

check_syntax

check_with_binary "coding-standard" "phpcs" \
    "vendor/bin/phpcs --standard=Magento2 --extensions=php,phtml $(files_argument)" \
    "magento/magento-coding-standard is not installed"

check_with_binary "static-analysis" "phpstan" \
    "vendor/bin/phpstan analyse --no-progress --error-format=raw $(files_argument)" \
    "phpstan/phpstan is not installed"

check_with_binary "formatting" "php-cs-fixer" \
    "vendor/bin/php-cs-fixer fix --dry-run --diff $(files_argument)" \
    "friendsofphp/php-cs-fixer is not installed"

if $run_everything; then
    check_with_binary "unit-tests" "phpunit" \
        "vendor/bin/phpunit -c dev/tests/unit/phpunit.xml.dist" \
        "phpunit/phpunit is not installed"

    output=$(in_container "bin/magento setup:di:compile")
    if [ $? -eq 0 ]; then
        record "di-compile" "ok" "0" ""
    else
        record "di-compile" "failed" "1" "$output"
    fi
else
    record "unit-tests" "skipped" "0" "slow: run with --all"
    record "di-compile" "skipped" "0" "slow: run with --all"
fi

# ------------------------------------------------------------------ report

failed=$(printf "$RESULTS" | awk -F'\t' '$2 == "failed"' | grep -c . || true)
ran=$(printf "$RESULTS" | awk -F'\t' '$2 != "skipped"' | grep -c . || true)

if is_json_output; then
    json_success "verify" "$(printf "$RESULTS" | jq -R -s --arg scope "$SCOPE" --argjson failed "${failed:-0}" '
        {scope: $scope, failed: $failed,
         checks: (split("\n") | map(select(length > 0) | split("\t") |
            {name: .[0], status: .[1], problems: (.[2] | tonumber),
             output: ((.[3] // "") | gsub("\u001e"; "\n"))}))}')"
    [ "${failed:-0}" -eq 0 ] && exit 0 || exit "$HM_EXIT_ERROR"
fi

printf '\n'
print_heading "Verifying $COMPOSE_PROJECT_NAME ($SCOPE)\n\n"

printf "$RESULTS" | while IFS=$'\t' read -r name status problems output; do
    [ -z "$name" ] && continue
    case "$status" in
        ok)      printf '  %b✓%b  %-18s\n' "${GREEN:-}" "${COLOR_RESET:-}" "$name" ;;
        failed)  printf '  %b✗%b  %-18s %s problem(s)\n' "${RED:-}" "${COLOR_RESET:-}" "$name" "$problems" ;;
        skipped) printf '  %b–%b  %-18s %s\n' "${WHITE:-}" "${COLOR_RESET:-}" "$name" "$output" ;;
    esac
done

if [ "${failed:-0}" -gt 0 ]; then
    printf '\n'
    printf "$RESULTS" | while IFS=$'\t' read -r name status problems output; do
        [ "$status" == "failed" ] || continue
        print_warning "$name\n"
        printf '%b\n' "$output" | head -30 | sed 's/^/    /'
        printf '\n'
    done
fi

if [ "${ran:-0}" -eq 0 ]; then
    print_info "Nothing to run: this project has no verification tools installed.\n\n"
    exit 0
fi

printf '\n'
[ "${failed:-0}" -eq 0 ] && exit 0 || exit "$HM_EXIT_ERROR"
