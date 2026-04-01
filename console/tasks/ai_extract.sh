#!/usr/bin/env bash
#
# AI Tools Extraction and Installation
# Handles validation, extraction, and atomic installation of skills/agents
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

# Include guard
[[ -n "${__AI_EXTRACT_SH__:-}" ]] && return 0
readonly __AI_EXTRACT_SH__=1

set -euo pipefail

# Load dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../components/print_message.sh"
source "${SCRIPT_DIR}/ai_registration.sh"

#
# Validate repository structure
# Args: $1 = repository root directory
# Returns: 0 if valid (has skills/ or agents/), 1 otherwise
#
validate_repository_structure() {
    local repo_dir="$1"

    if [[ ! -d "${repo_dir}" ]]; then
        print_error "Repository directory does not exist: ${repo_dir}"
        return 1
    fi

    # Check for skills/ or agents/ directories
    if [[ -d "${repo_dir}/skills" ]] || [[ -d "${repo_dir}/agents" ]]; then
        return 0
    fi

    print_warning "Repository has no skills/ or agents/ directories"
    return 1
}

#
# Extract tarball with validation
# Args: $1 = tarball file path,
#       $2 = output directory
# Returns: 0 on success, 1 on failure
#
extract_tarball() {
    local tarball="$1"
    local output_dir="$2"

    if [[ ! -f "${tarball}" ]]; then
        print_error "Tarball not found: ${tarball}"
        return 1
    fi

    # Create output directory
    mkdir -p "${output_dir}" || {
        print_error "Failed to create output directory: ${output_dir}"
        return 1
    }

    # Extract with validation
    if ! tar -xzf "${tarball}" -C "${output_dir}" 2>/dev/null; then
        print_error "Failed to extract tarball"
        return 1
    fi

    return 0
}

#
# Install skills/agents from repository with atomic operations
# Args: $1 = source repository directory,
#       $2 = resource type (skills|agents),
#       $3 = target platform directory (e.g., .claude/skills),
#       $4 = force overwrite (true|false, default: false)
# Returns: 0 on success, 1 on failure
# Side effects: Updates ai-registration.json
#
install_from_repository() {
    local repo_dir="$1"
    local resource_type="$2"
    local platform_dir="$3"
    local force_overwrite="${4:-false}"

    # Validate resource type
    if [[ "${resource_type}" != "skills" && "${resource_type}" != "agents" ]]; then
        print_error "Invalid resource type: ${resource_type}"
        return 1
    fi

    # Validate repository structure
    validate_repository_structure "${repo_dir}" || return 1

    local source_dir="${repo_dir}/${resource_type}"

    # Check if source directory exists
    if [[ ! -d "${source_dir}" ]]; then
        print_info "No ${resource_type} found in repository"
        return 0
    fi

    # Create target directory
    mkdir -p "${platform_dir}" || {
        print_error "Failed to create target directory: ${platform_dir}"
        return 1
    }

    # Create temp directory for atomic operations
    local temp_dir
    temp_dir=$(mktemp -d "${platform_dir}/.tmp.XXXXXX")

    # Track installed count
    local installed_count=0
    local skipped_count=0

    # Install each skill/agent directory
    for item_path in "${source_dir}"/*; do
        if [[ ! -d "${item_path}" ]]; then
            continue
        fi

        local item_name
        item_name=$(basename "${item_path}")
        local target_path="${platform_dir}/${item_name}"
        local temp_path="${temp_dir}/${item_name}"

        # Check for conflicts
        if [[ -d "${target_path}" ]]; then
            # Check if already tracked (downloaded by hm ai-*)
            if is_tracked "${resource_type}" "${target_path}"; then
                # Tracked file - safe to overwrite
                print_info "Updating ${item_name}..."
            else
                # Custom file - handle based on force flag
                if [[ "${force_overwrite}" == "true" ]]; then
                    print_warning "Overwriting custom ${resource_type%s}: ${item_name} (--force enabled)"
                else
                    print_warning "Skipping ${item_name} (custom ${resource_type%s} exists, use --force to overwrite)"
                    ((skipped_count++))
                    continue
                fi
            fi
        else
            print_info "Installing ${item_name}..."
        fi

        # Copy to temp directory first
        cp -r "${item_path}" "${temp_path}" || {
            print_error "Failed to copy ${item_name}"
            rm -rf "${temp_dir}"
            return 1
        }

        # Atomic move from temp to target
        if [[ -d "${target_path}" ]]; then
            rm -rf "${target_path}"
        fi

        mv "${temp_path}" "${target_path}" || {
            print_error "Failed to install ${item_name}"
            rm -rf "${temp_dir}"
            return 1
        }

        # Register installation
        add_registration_entry "${resource_type}" "${target_path}" || {
            print_warning "Failed to register ${item_name} (continuing anyway)"
        }

        ((installed_count++))
    done

    # Clean up temp directory
    rm -rf "${temp_dir}"

    # Report results
    if [[ ${installed_count} -eq 0 ]] && [[ ${skipped_count} -eq 0 ]]; then
        print_info "No ${resource_type} to install"
    else
        print_info "Installed ${installed_count} ${resource_type}"
        if [[ ${skipped_count} -gt 0 ]]; then
            print_info "Skipped ${skipped_count} custom ${resource_type}"
        fi
    fi

    return 0
}

#
# Filter and install specific skill types
# Args: $1 = source repository directory,
#       $2 = resource type (skills|agents),
#       $3 = target platform directory,
#       $4 = comma-separated list of skill types (e.g., "hyva,magento"),
#       $5 = force overwrite (true|false, default: false)
# Returns: 0 on success, 1 on failure
#
install_filtered() {
    local repo_dir="$1"
    local resource_type="$2"
    local platform_dir="$3"
    local skill_types="$4"
    local force_overwrite="${5:-false}"

    # If no skill types filter, install everything
    if [[ -z "${skill_types}" ]]; then
        install_from_repository "${repo_dir}" "${resource_type}" "${platform_dir}" "${force_overwrite}"
        return $?
    fi

    # Load skill type definitions
    local skill_types_json
    if [[ ! -f "data/ai-skill-types.json" ]]; then
        print_error "Skill types configuration not found"
        return 1
    fi
    skill_types_json=$(cat "data/ai-skill-types.json")

    # Convert comma-separated list to array
    IFS=',' read -ra types_array <<< "${skill_types}"

    # Validate repository structure
    validate_repository_structure "${repo_dir}" || return 1

    local source_dir="${repo_dir}/${resource_type}"

    if [[ ! -d "${source_dir}" ]]; then
        print_info "No ${resource_type} found in repository"
        return 0
    fi

    # Create target directory
    mkdir -p "${platform_dir}" || {
        print_error "Failed to create target directory: ${platform_dir}"
        return 1
    }

    # Create temp directory for atomic operations
    local temp_dir
    temp_dir=$(mktemp -d "${platform_dir}/.tmp.XXXXXX")

    local installed_count=0
    local skipped_count=0

    # Install items matching skill types
    for item_path in "${source_dir}"/*; do
        if [[ ! -d "${item_path}" ]]; then
            continue
        fi

        local item_name
        item_name=$(basename "${item_path}")

        # Check if item matches any requested skill type
        local should_install=false
        for skill_type in "${types_array[@]}"; do
            # Simple prefix matching (e.g., hyva-* matches type "hyva")
            if [[ "${item_name}" =~ ^${skill_type}- ]] || [[ "${item_name}" == "${skill_type}" ]]; then
                should_install=true
                break
            fi
        done

        if [[ "${should_install}" == "false" ]]; then
            continue
        fi

        local target_path="${platform_dir}/${item_name}"
        local temp_path="${temp_dir}/${item_name}"

        # Check for conflicts
        if [[ -d "${target_path}" ]]; then
            if is_tracked "${resource_type}" "${target_path}"; then
                print_info "Updating ${item_name}..."
            else
                if [[ "${force_overwrite}" == "true" ]]; then
                    print_warning "Overwriting custom ${resource_type%s}: ${item_name} (--force enabled)"
                else
                    print_warning "Skipping ${item_name} (custom ${resource_type%s} exists)"
                    ((skipped_count++))
                    continue
                fi
            fi
        else
            print_info "Installing ${item_name}..."
        fi

        # Copy to temp, then atomic move
        cp -r "${item_path}" "${temp_path}" || {
            print_error "Failed to copy ${item_name}"
            rm -rf "${temp_dir}"
            return 1
        }

        if [[ -d "${target_path}" ]]; then
            rm -rf "${target_path}"
        fi

        mv "${temp_path}" "${target_path}" || {
            print_error "Failed to install ${item_name}"
            rm -rf "${temp_dir}"
            return 1
        }

        add_registration_entry "${resource_type}" "${target_path}" || {
            print_warning "Failed to register ${item_name}"
        }

        ((installed_count++))
    done

    # Clean up
    rm -rf "${temp_dir}"

    # Report
    if [[ ${installed_count} -eq 0 ]] && [[ ${skipped_count} -eq 0 ]]; then
        print_info "No matching ${resource_type} found for types: ${skill_types}"
    else
        print_info "Installed ${installed_count} ${resource_type}"
        if [[ ${skipped_count} -gt 0 ]]; then
            print_info "Skipped ${skipped_count} custom ${resource_type}"
        fi
    fi

    return 0
}
