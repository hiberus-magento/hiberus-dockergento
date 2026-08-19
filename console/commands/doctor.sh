#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/doctor.sh
source "$HELPERS_DIR"/docker.sh

DOCTOR_DIR="$TASKS_DIR/doctor"
CHECK_TIMEOUT=5
only=""

#
# Is this directory a configured project?
#
detect_project() {
    if [ -f "$DOCKER_COMPOSE_FILE" ] && [ -f "$CUSTOM_PROPERTIES_DIR/properties.json" ]; then
        echo "true"
    else
        echo "false"
    fi
}

#
# Synthesise a result for a check that could not report one itself
#
synthetic_result() {
    printf '%s%s%s%s%s%s%s%s%s\n' \
        "$1" "$HM_DOCTOR_SEPARATOR" "global" "$HM_DOCTOR_SEPARATOR" \
        "warning" "$HM_DOCTOR_SEPARATOR" "$2" "$HM_DOCTOR_SEPARATOR" \
        "$COMMAND_BIN_NAME doctor --only=$1"
}

#
# Run one check with a time limit, in its own process
#
run_check() {
    local script="$1"
    local id="$2"
    local output status=0

    output=$(HM_DOCTOR_ID="$id" perl -e 'alarm shift; exec @ARGV' \
        "$CHECK_TIMEOUT" bash "$script" 2>/dev/null) || status=$?

    if [ "$status" -eq 142 ]; then
        synthetic_result "$id" "Check timed out after ${CHECK_TIMEOUT}s"
        return 0
    fi

    if [ "$status" -ne 0 ] && [ -z "$output" ]; then
        synthetic_result "$id" "Check failed unexpectedly (exit $status)"
        return 0
    fi

    if [ -n "$output" ]; then
        printf '%s\n' "$output"
    fi

    return 0
}

#
# Run every check and print the raw result lines
#
collect_results() {
    local script id

    export HM_DOCTOR_IN_PROJECT
    HM_DOCTOR_IN_PROJECT=$(detect_project)

    # The container table costs a Docker round trip: load it once and hand it down
    hm_load_container_table
    export HM_DOCTOR_CONTAINER_TABLE="$HM_CONTAINER_TABLE_CACHE"

    for script in "$DOCTOR_DIR"/*.sh; do
        [ -f "$script" ] || continue

        id=$(basename "$script" .sh)
        id="${id#*-}"

        if [ -n "$only" ] && [ "$id" != "$only" ]; then
            continue
        fi

        run_check "$script" "$id"
    done
}

#
# Readable report
#
print_text() {
    local results="$1"
    local id scope severity message action
    local errors=0 warnings=0

    printf "\n"

    while IFS="$HM_DOCTOR_SEPARATOR" read -r id scope severity message action; do
        [ -z "$id" ] && continue

        case "$severity" in
            ok)      print_info "  OK   " ;;
            warning) print_warning "  WARN " ; warnings=$((warnings + 1)) ;;
            error)   print_error "  FAIL " ; errors=$((errors + 1)) ;;
        esac

        printf "%-20s %s\n" "$id" "$message"

        if [ -n "$action" ] && [ "$severity" != "ok" ]; then
            printf "       %-18s " ""
            print_code "$action\n"
        fi
    done <<< "$results"

    printf "\n"

    if [ "$errors" -gt 0 ]; then
        print_error "  $errors error(s), $warnings warning(s)\n\n"
    elif [ "$warnings" -gt 0 ]; then
        print_warning "  No blocking problems, $warnings warning(s)\n\n"
    else
        print_info "  Everything looks good\n\n"
    fi
}

doctor_execute() {
    local results errors data

    results=$(collect_results)
    errors=$(printf '%s\n' "$results" |
        awk -F"$HM_DOCTOR_SEPARATOR" '$3 == "error"' | sed '/^$/d' | wc -l | tr -d ' ')

    if is_json_output; then
        data=$(printf '%s\n' "$results" | jq -R -s -c '
            split("\n")
            | map(select(length > 0) | split("\u001f") | {
                id: .[0], scope: .[1], severity: .[2], message: .[3], action: .[4]
              })
            | { checks: ., summary: {
                  total: length,
                  ok: (map(select(.severity == "ok")) | length),
                  warnings: (map(select(.severity == "warning")) | length),
                  errors: (map(select(.severity == "error")) | length)
              } }')
        json_success "doctor" "$data"
    else
        print_text "$results"
    fi

    if [ "$errors" -gt 0 ]; then
        exit "$HM_EXIT_ERROR"
    fi

    exit "$HM_EXIT_OK"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --only=*)
            only="${1#*=}"
            shift
            ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                "Unknown option: $1" \
                "$COMMAND_BIN_NAME doctor --help"
            ;;
    esac
done

doctor_execute
