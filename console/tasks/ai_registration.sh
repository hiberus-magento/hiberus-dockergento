#!/usr/bin/env bash
#
# AI Registration Management
# Handles ai-properties.json and ai-registration.json I/O with atomic writes
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

# Include guard
[[ -n "${__AI_REGISTRATION_SH__:-}" ]] && return 0
readonly __AI_REGISTRATION_SH__=1

set -euo pipefail

# Load dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../components/print_message.sh"
source "${SCRIPT_DIR}/../helpers/properties.sh"

# Configuration file paths
readonly AI_PROPERTIES_FILE="config/docker/ai-properties.json"
readonly AI_REGISTRATION_FILE="config/docker/ai-registration.json"

#
# Load AI properties configuration
# Returns: JSON content or empty object if file doesn't exist
# Exit: 1 if file exists but is invalid JSON
#
load_ai_properties() {
    local config_file="${AI_PROPERTIES_FILE}"

    if [[ ! -f "${config_file}" ]]; then
        echo "{}"
        return 0
    fi

    if ! jq empty "${config_file}" 2>/dev/null; then
        print_error "Invalid JSON in ${config_file}"
        return 1
    fi

    cat "${config_file}"
}

#
# Save AI properties configuration with atomic write
# Args: $1 = JSON content to save
# Exit: 1 on validation or write failure
#
save_ai_properties() {
    local json_content="$1"
    local config_file="${AI_PROPERTIES_FILE}"
    local config_dir
    config_dir="$(dirname "${config_file}")"

    # Ensure directory exists
    if [[ ! -d "${config_dir}" ]]; then
        mkdir -p "${config_dir}" || {
            print_error "Failed to create directory: ${config_dir}"
            return 1
        }
    fi

    # Validate JSON before writing
    if ! echo "${json_content}" | jq empty 2>/dev/null; then
        print_error "Invalid JSON content provided"
        return 1
    fi

    # Pretty-print JSON
    local formatted_json
    formatted_json=$(echo "${json_content}" | jq .)

    # Atomic write: temp file → validate → move
    local temp_file
    temp_file=$(mktemp "${config_file}.XXXXXX")

    echo "${formatted_json}" > "${temp_file}" || {
        rm -f "${temp_file}"
        print_error "Failed to write temporary file"
        return 1
    }

    # Atomic move
    mv "${temp_file}" "${config_file}" || {
        rm -f "${temp_file}"
        print_error "Failed to save ${config_file}"
        return 1
    }

    return 0
}

#
# Load AI registration tracking data
# Returns: JSON content or empty object if file doesn't exist
# Exit: 1 if file exists but is invalid JSON
#
load_ai_registration() {
    local reg_file="${AI_REGISTRATION_FILE}"

    if [[ ! -f "${reg_file}" ]]; then
        echo '{"skills":{},"agents":{}}'
        return 0
    fi

    if ! jq empty "${reg_file}" 2>/dev/null; then
        print_error "Invalid JSON in ${reg_file}"
        print_warning "Registration file is corrupted - refusing to operate"
        return 1
    fi

    cat "${reg_file}"
}

#
# Save AI registration tracking data with atomic write
# Args: $1 = JSON content to save
# Exit: 1 on validation or write failure
#
save_ai_registration() {
    local json_content="$1"
    local reg_file="${AI_REGISTRATION_FILE}"
    local reg_dir
    reg_dir="$(dirname "${reg_file}")"

    # Ensure directory exists
    if [[ ! -d "${reg_dir}" ]]; then
        mkdir -p "${reg_dir}" || {
            print_error "Failed to create directory: ${reg_dir}"
            return 1
        }
    fi

    # Validate JSON structure (must have skills and agents keys)
    if ! echo "${json_content}" | jq -e '.skills and .agents' >/dev/null 2>&1; then
        print_error "Invalid registration JSON structure (missing skills/agents keys)"
        return 1
    fi

    # Pretty-print JSON
    local formatted_json
    formatted_json=$(echo "${json_content}" | jq .)

    # Atomic write: temp file → validate → move
    local temp_file
    temp_file=$(mktemp "${reg_file}.XXXXXX")

    echo "${formatted_json}" > "${temp_file}" || {
        rm -f "${temp_file}"
        print_error "Failed to write temporary file"
        return 1
    }

    # Atomic move
    mv "${temp_file}" "${reg_file}" || {
        rm -f "${temp_file}"
        print_error "Failed to save ${reg_file}"
        return 1
    }

    return 0
}

#
# Add file entry to registration tracking
# Args: $1 = resource type (skills|agents), $2 = file path, $3 = optional SHA256
# Exit: 0 on success, 1 on failure
#
add_registration_entry() {
    local resource_type="$1"
    local file_path="$2"
    local checksum="${3:-}"

    if [[ "${resource_type}" != "skills" && "${resource_type}" != "agents" ]]; then
        print_error "Invalid resource type: ${resource_type} (must be skills or agents)"
        return 1
    fi

    # Calculate checksum if not provided
    if [[ -z "${checksum}" ]] && [[ -f "${file_path}" ]]; then
        checksum=$(sha256sum "${file_path}" 2>/dev/null | awk '{print $1}' || echo "")
    fi

    # Load current registration
    local registration
    registration=$(load_ai_registration) || return 1

    # Add entry
    local updated_registration
    updated_registration=$(echo "${registration}" | jq \
        --arg type "${resource_type}" \
        --arg path "${file_path}" \
        --arg sha "${checksum}" \
        '.[$type][$path] = {"checksum": $sha, "installed": (now | strftime("%Y-%m-%d %H:%M:%S"))}')

    # Save updated registration
    save_ai_registration "${updated_registration}"
}

#
# Remove file entry from registration tracking
# Args: $1 = resource type (skills|agents), $2 = file path
# Exit: 0 on success, 1 on failure
#
remove_registration_entry() {
    local resource_type="$1"
    local file_path="$2"

    if [[ "${resource_type}" != "skills" && "${resource_type}" != "agents" ]]; then
        print_error "Invalid resource type: ${resource_type}"
        return 1
    fi

    # Load current registration
    local registration
    registration=$(load_ai_registration) || return 1

    # Remove entry
    local updated_registration
    updated_registration=$(echo "${registration}" | jq \
        --arg type "${resource_type}" \
        --arg path "${file_path}" \
        'del(.[$type][$path])')

    # Save updated registration
    save_ai_registration "${updated_registration}"
}

#
# Check if file is tracked in registration
# Args: $1 = resource type (skills|agents), $2 = file path
# Returns: 0 if tracked, 1 if not tracked or error
#
is_tracked() {
    local resource_type="$1"
    local file_path="$2"

    # Load current registration
    local registration
    registration=$(load_ai_registration) || return 1

    # Check if entry exists
    echo "${registration}" | jq -e \
        --arg type "${resource_type}" \
        --arg path "${file_path}" \
        '.[$type][$path] != null' >/dev/null 2>&1
}

#
# Get all tracked files for a resource type
# Args: $1 = resource type (skills|agents)
# Returns: Newline-separated list of file paths
# Exit: 1 on failure
#
get_tracked_files() {
    local resource_type="$1"

    if [[ "${resource_type}" != "skills" && "${resource_type}" != "agents" ]]; then
        print_error "Invalid resource type: ${resource_type}"
        return 1
    fi

    # Load current registration
    local registration
    registration=$(load_ai_registration) || return 1

    # Extract file paths
    echo "${registration}" | jq -r \
        --arg type "${resource_type}" \
        '.[$type] | keys[]' 2>/dev/null || true
}

#
# Clear all registration tracking (for ai-reset)
# Exit: 0 on success, 1 on failure
#
clear_registration() {
    local empty_registration='{"skills":{},"agents":{}}'
    save_ai_registration "${empty_registration}"
}
