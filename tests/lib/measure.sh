#!/usr/bin/env bash
#
# Timing helpers for the performance suite.
#
# Wall clock through python3 rather than bash's SECONDS: the numbers here are hundreds of
# milliseconds and second-level resolution would be useless.
#

#
# Milliseconds taken by a command: measure_ms <command...>
#
measure_ms() {
    local start end
    start=$(python3 -c 'import time; print(time.time())')
    "$@" >/dev/null 2>&1 || true
    end=$(python3 -c 'import time; print(time.time())')
    python3 -c "print(int(($end - $start) * 1000))"
}

#
# Best of three runs, in milliseconds: a single measurement on a machine with Docker and a
# cold page cache is noise, and the best run is the one that reflects the code rather than
# whatever else the laptop was doing.
#
measure_best_ms() {
    local best="" current
    local attempt

    for attempt in 1 2 3; do
        current=$(measure_ms "$@")

        if [ -z "$best" ] || [ "$current" -lt "$best" ]; then
            best="$current"
        fi
    done

    echo "$best"
}

#
# assert_faster_than <budget_ms> <label> <command...>
#
assert_faster_than() {
    local budget="$1"
    local label="$2"
    shift 2

    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    HM_CURRENT_TEST="$label"

    local elapsed
    elapsed=$(measure_best_ms "$@")

    if [ "$elapsed" -le "$budget" ]; then
        printf '  \033[0;32m✓\033[0m %s \033[0;90m(%s ms, budget %s ms)\033[0m\n' \
            "$label" "$elapsed" "$budget"
    else
        HM_TESTS_FAILED=$((HM_TESTS_FAILED + 1))
        printf '  \033[0;31m✗\033[0m %s\n' "$label"
        printf '      took %s ms, budget is %s ms\n' "$elapsed" "$budget"
    fi
}

#
# Count the processes a command spawns, by shadowing them in the PATH:
#   count_spawned <binary> <command...>
#
count_spawned() {
    local binary="$1"
    shift

    local real shim log
    real=$(command -v "$binary") || { echo "0"; return 0; }
    shim=$(mktemp -d)
    log="$shim/calls.log"
    : > "$log"

    cat > "$shim/$binary" <<SHIM
#!/bin/bash
echo call >> "$log"
exec "$real" "\$@"
SHIM
    chmod +x "$shim/$binary"

    PATH="$shim:$PATH" "$@" >/dev/null 2>&1 || true

    wc -l < "$log" | tr -d ' '
    rm -rf "$shim"
}
