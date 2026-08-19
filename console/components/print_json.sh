#!/usr/bin/env bash

#
# Structured output component.
#
# Every JSON response shares the same envelope so that consumers (TUI, dashboard,
# AI agents) can rely on a stable shape:
#
#   success -> stdout: {"schema_version":1,"command":"describe","ok":true,"data":{...}}
#   error   -> stderr: {"schema_version":1,"command":"start","ok":false,
#                       "error":{"code":3,"type":"docker_unavailable",
#                                "message":"...","hint":"..."}}
#
# All values are built with `jq -n --arg` so quoting, newlines and UTF-8 are escaped
# correctly instead of being pasted into a hand-written string.
#

HM_SCHEMA_VERSION="${HM_SCHEMA_VERSION:-1}"

#
# True when the active output format is JSON
#
is_json_output() {
    [[ "${HM_OUTPUT_FORMAT:-text}" == "json" ]]
}

#
# Build a JSON object from key/value pairs: json_object key1 value1 key2 value2 ...
# Values are always emitted as strings; use json_object_raw for typed values.
#
json_object() {
    local jq_args=() filter="" index=0

    while [ "$#" -gt 1 ]; do
        jq_args+=(--arg "k${index}" "$1" --arg "v${index}" "$2")
        [ -n "$filter" ] && filter+=" + "
        filter+="{(\$k${index}): \$v${index}}"
        shift 2
        index=$((index + 1))
    done

    if [ -z "$filter" ]; then
        echo "{}"
        return 0
    fi

    jq -n "${jq_args[@]}" "$filter"
}

#
# Build a JSON object from key/JSON-value pairs, for values that are not strings
# (numbers, booleans, nested objects, arrays)
#
json_object_raw() {
    local jq_args=() filter="" index=0

    while [ "$#" -gt 1 ]; do
        jq_args+=(--arg "k${index}" "$1" --argjson "v${index}" "$2")
        [ -n "$filter" ] && filter+=" + "
        filter+="{(\$k${index}): \$v${index}}"
        shift 2
        index=$((index + 1))
    done

    if [ -z "$filter" ]; then
        echo "{}"
        return 0
    fi

    jq -n "${jq_args[@]}" "$filter"
}

#
# Emit a success envelope on stdout: json_success <command> [<data-json>]
#
json_success() {
    local command_name="$1"
    local data="${2:-\{\}}"

    jq -n \
        --argjson schema_version "$HM_SCHEMA_VERSION" \
        --arg command "$command_name" \
        --argjson data "$data" \
        '{schema_version: $schema_version, command: $command, ok: true, data: $data}'
}

#
# Emit an error envelope on stderr: json_error <command> <code> <type> <message> [hint]
#
json_error() {
    local command_name="$1"
    local code="$2"
    local type="$3"
    local message="$4"
    local hint="${5:-}"

    jq -n \
        --argjson schema_version "$HM_SCHEMA_VERSION" \
        --arg command "$command_name" \
        --argjson code "$code" \
        --arg type "$type" \
        --arg message "$message" \
        --arg hint "$hint" \
        '{schema_version: $schema_version, command: $command, ok: false,
          error: {code: $code, type: $type, message: $message, hint: $hint}}' >&2
}
