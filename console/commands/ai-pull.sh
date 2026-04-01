#!/usr/bin/env bash
#
# AI Tools Pull Command
# Update skills/agents from configured repositories
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

set -euo pipefail

# Load dependencies
source "${COMPONENTS_DIR}/print_message.sh"
source "${TASKS_DIR}/ai_registration.sh"
source "${TASKS_DIR}/ai_download.sh"
source "${TASKS_DIR}/ai_extract.sh"

#
# Parse command-line options
# Sets global variables: OPT_FORCE
#
parse_options() {
    OPT_FORCE="false"

    for arg in "$@"; do
        case "${arg}" in
            --force)
                OPT_FORCE="true"
                ;;
            *)
                print_error "Unknown option: ${arg}"
                show_usage
                exit 1
                ;;
        esac
    done
}

#
# Show command usage
#
show_usage() {
    cat <<'EOF'
Usage: hm ai-pull [OPTIONS]

Download/update AI tools (skills/agents) from configured repositories.

Options:
  --force    Force re-download and overwrite existing skills/agents (including custom)

Prerequisites:
  - ai-properties.json must exist (run 'hm ai-init' first)

Behavior:
  - Reads configuration from config/docker/ai-properties.json
  - Downloads from all configured repositories (default + custom)
  - Preserves custom skills/agents (unless --force is used)
  - Updates ai-registration.json with tracked files

Examples:
  hm ai-pull
  hm ai-pull --force

EOF
}

#
# Resolve repositories to download from
# Args: $1 = config JSON
# Returns: Array of repository objects via stdout
#
resolve_repositories() {
    local config="$1"

    # Load default repositories
    local default_repos="[]"
    if [[ -f "data/ai-repositories.json" ]]; then
        default_repos=$(jq -c '.repositories' data/ai-repositories.json)
    fi

    # Get custom repositories from config
    local custom_repos
    custom_repos=$(echo "${config}" | jq -c '.custom_repositories // []')

    # Merge arrays
    jq -n \
        --argjson default "${default_repos}" \
        --argjson custom "${custom_repos}" \
        '$default + $custom'
}

#
# Download and install tools from repositories
# Args: $1 = repositories JSON array,
#       $2 = platforms (comma-separated),
#       $3 = types (comma-separated),
#       $4 = resources (comma-separated),
#       $5 = force overwrite (true|false)
# Returns: 0 on success, 1 on failure
#
download_and_install() {
    local repositories="$1"
    local platforms="$2"
    local types="$3"
    local resources="$4"
    local force="$5"

    # Load platform definitions
    local platforms_json
    platforms_json=$(cat data/ai-platforms.json)

    local repo_count
    repo_count=$(echo "${repositories}" | jq 'length')

    if [[ ${repo_count} -eq 0 ]]; then
        print_info "No repositories configured"
        return 0
    fi

    # Create temp directory for downloads
    local temp_base
    temp_base=$(mktemp -d "/tmp/hm-ai-tools.XXXXXX")

    print_info "Downloading from ${repo_count} repositories..."

    local success_count=0
    local fail_count=0

    # Iterate repositories
    for ((i=0; i<repo_count; i++)); do
        local repo_name repo_url repo_branch

        repo_name=$(echo "${repositories}" | jq -r ".[$i].name")
        repo_url=$(echo "${repositories}" | jq -r ".[$i].url")
        repo_branch=$(echo "${repositories}" | jq -r ".[$i].branch // \"main\"")

        print_info "Processing repository: ${repo_name}"

        # Download repository
        local repo_dir="${temp_base}/${repo_name}"
        if ! download_repository "${repo_url}" "${repo_branch}" "${repo_dir}"; then
            print_warning "Failed to download ${repo_name}, skipping..."
            ((fail_count++))
            continue
        fi

        # Validate structure
        if ! validate_repository_structure "${repo_dir}"; then
            print_warning "Invalid repository structure in ${repo_name}, skipping..."
            ((fail_count++))
            continue
        fi

        # Install for each platform + resource combination
        IFS=',' read -ra platform_array <<< "${platforms}"
        IFS=',' read -ra resource_array <<< "${resources}"

        local repo_success=false

        for platform in "${platform_array[@]}"; do
            for resource in "${resource_array[@]}"; do
                local dir_key="${resource}_dir"
                local target_dir

                target_dir=$(echo "${platforms_json}" | jq -r --arg p "${platform}" --arg k "${dir_key}" '.platforms[$p][$k] // empty')

                if [[ -z "${target_dir}" ]]; then
                    continue
                fi

                print_info "Installing ${resource} for ${platform}..."

                # Install with type filtering
                if install_filtered "${repo_dir}" "${resource}" "${target_dir}" "${types}" "${force}"; then
                    repo_success=true
                fi
            done
        done

        if [[ "${repo_success}" == "true" ]]; then
            ((success_count++))
        else
            ((fail_count++))
        fi
    done

    # Clean up temp directory
    rm -rf "${temp_base}"

    # Report results
    print_info "Processed ${repo_count} repositories (${success_count} successful, ${fail_count} failed)"

    if [[ ${fail_count} -gt 0 ]]; then
        print_warning "Some repositories failed to download or install"
        return 1
    fi

    return 0
}

#
# Main execution
#
main() {
    # Parse options
    parse_options "$@"

    # Load configuration
    print_info "Loading configuration..."
    local config
    config=$(load_ai_properties) || {
        print_error "Failed to load configuration"
        exit 1
    }

    # Check if configuration exists
    if [[ "${config}" == "{}" ]]; then
        print_error "No configuration found. Run 'hm ai-init' first."
        exit 1
    fi

    # Validate configuration structure
    if ! echo "${config}" | jq -e '.platforms and .resources' >/dev/null 2>&1; then
        print_error "Invalid configuration in ai-properties.json"
        exit 1
    fi

    # Extract configuration values
    local platforms types resources
    platforms=$(echo "${config}" | jq -r '.platforms | join(",")')
    types=$(echo "${config}" | jq -r '.types | join(",")')
    resources=$(echo "${config}" | jq -r '.resources | join(",")')

    if [[ -z "${platforms}" ]] || [[ -z "${resources}" ]]; then
        print_error "Configuration incomplete: platforms and resources are required"
        exit 1
    fi

    print_info "Configuration loaded:"
    print_info "  Platforms: ${platforms}"
    print_info "  Types: ${types:-all}"
    print_info "  Resources: ${resources}"
    echo ""

    # Resolve repositories
    print_info "Resolving repositories..."
    local repositories
    repositories=$(resolve_repositories "${config}") || {
        print_error "Failed to resolve repositories"
        exit 1
    }

    # Download and install
    print_info "Starting download and installation..."
    if ! download_and_install "${repositories}" "${platforms}" "${types}" "${resources}" "${OPT_FORCE}"; then
        print_warning "Some operations failed, but continuing..."
    fi

    print_info "AI tools update complete!"
    echo ""
    print_info "Registration: config/docker/ai-registration.json"
    echo ""

    if [[ "${OPT_FORCE}" == "true" ]]; then
        print_warning "Used --force flag: custom skills/agents may have been overwritten"
    else
        print_info "Custom skills/agents were preserved"
        print_info "Use --force to overwrite custom files"
    fi
}

main "$@"
