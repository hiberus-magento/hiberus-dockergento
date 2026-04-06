#!/usr/bin/env bash
#
# AI Tools Initialization Command
# Interactive wizard to configure AI platforms, skill types, and download tools
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

set -euo pipefail

# Load dependencies
source "${COMPONENTS_DIR}/print_message.sh"
source "${TASKS_DIR}/ai_wizard.sh"
source "${TASKS_DIR}/ai_registration.sh"
source "${TASKS_DIR}/ai_download.sh"
source "${TASKS_DIR}/ai_extract.sh"

#
# Parse command-line options
# Sets global variables: OPT_PLATFORMS, OPT_TYPES, OPT_RESOURCES, OPT_REPOSITORY, OPT_BRANCH
#
parse_options() {
    OPT_PLATFORMS=""
    OPT_TYPES=""
    OPT_RESOURCES=""
    OPT_REPOSITORY=""
    OPT_BRANCH="main"

    for arg in "$@"; do
        case "${arg}" in
            --platforms=*)
                OPT_PLATFORMS="${arg#*=}"
                ;;
            --types=*)
                OPT_TYPES="${arg#*=}"
                ;;
            --resources=*)
                OPT_RESOURCES="${arg#*=}"
                ;;
            --repository=*)
                OPT_REPOSITORY="${arg#*=}"
                ;;
            --branch=*)
                OPT_BRANCH="${arg#*=}"
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
Usage: hm ai-init [OPTIONS]

Initialize AI tools configuration via interactive wizard and download skills/agents.

Options:
  --platforms=<list>    Comma-separated AI platforms (claude,cursor,codex,copilot,gemini,opencode)
  --types=<list>        Comma-separated skill types (hyva,acs,magento,php)
  --resources=<list>    Resource types to download (skills,agents,both)
  --repository=<url>    Custom repository URL (HTTPS only)
  --branch=<name>       Branch name for custom repository (default: main)

Examples:
  hm ai-init
  hm ai-init --platforms=claude,cursor --types=hyva,magento --resources=skills,agents
  hm ai-init --repository=https://github.com/org/repo --branch=main

Interactive mode:
  Run without options for an interactive wizard that guides you through configuration.

Reconfiguration:
  Run on a project with existing ai-properties.json to modify the configuration.
  The wizard will pre-fill existing selections.

EOF
}

#
# Build configuration from command-line options (non-interactive mode)
# Returns: Configuration JSON via stdout
#
build_config_from_options() {
    local resources="${OPT_RESOURCES}"
    local platforms="${OPT_PLATFORMS}"
    local types="${OPT_TYPES}"
    local custom_repos="[]"

    # Handle custom repository
    if [[ -n "${OPT_REPOSITORY}" ]]; then
        local repo_name
        repo_name=$(basename "${OPT_REPOSITORY}" .git)

        custom_repos=$(jq -n \
            --arg name "${repo_name}" \
            --arg url "${OPT_REPOSITORY}" \
            --arg branch "${OPT_BRANCH}" \
            '[{name: $name, url: $url, branch: $branch, types: []}]')
    fi

    # Default to "both" if resources not specified
    if [[ -z "${resources}" ]]; then
        resources="skills,agents"
    fi

    # Build config JSON
    jq -n \
        --arg resources "${resources}" \
        --arg platforms "${platforms}" \
        --arg types "${types}" \
        --argjson custom_repos "${custom_repos}" \
        '{
            resources: ($resources | split(",") | map(select(length > 0))),
            platforms: ($platforms | split(",") | map(select(length > 0))),
            types: ($types | split(",") | map(select(length > 0))),
            custom_repositories: $custom_repos,
            configured_at: (now | strftime("%Y-%m-%d %H:%M:%S"))
        }'
}

#
# Resolve repositories to download from
# Merges default repositories (data/ai-repositories.json) with custom repos
# Args: $1 = config JSON
# Returns: Array of repository objects via stdout
#
resolve_repositories() {
    local config="$1"

    # Load default repositories
    local default_repos="[]"
    local repos_file="${DATA_DIR}/ai-repositories.json"
    if [[ -f "${repos_file}" ]]; then
        default_repos=$(jq -c '.repositories' "${repos_file}")
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
# Create platform directories
# Args: $1 = comma-separated platform list, $2 = comma-separated resource list
#
create_platform_directories() {
    local platforms="$1"
    local resources="$2"

    # Load platform definitions
    local platforms_file="${DATA_DIR}/ai-platforms.json"
    if [[ ! -f "${platforms_file}" ]]; then
        print_error "Platform definitions not found: ${platforms_file}"
        return 1
    fi

    local platforms_json
    platforms_json=$(cat "${platforms_file}")

    IFS=',' read -ra platform_array <<< "${platforms}"
    IFS=',' read -ra resource_array <<< "${resources}"

    for platform in "${platform_array[@]}"; do
        for resource in "${resource_array[@]}"; do
            local dir_key="${resource}_dir"
            local target_dir

            target_dir=$(echo "${platforms_json}" | jq -r --arg p "${platform}" --arg k "${dir_key}" '.platforms[$p][$k] // empty')

            if [[ -n "${target_dir}" ]]; then
                mkdir -p "${target_dir}" || {
                    print_warning "Failed to create directory: ${target_dir}"
                }
            fi
        done
    done
}

#
# Download and install tools from repositories
# Args: $1 = repositories JSON array,
#       $2 = platforms (comma-separated),
#       $3 = types (comma-separated),
#       $4 = resources (comma-separated)
# Returns: 0 on success, 1 on failure
#
download_and_install() {
    local repositories="$1"
    local platforms="$2"
    local types="$3"
    local resources="$4"

    # Load platform definitions
    local platforms_json
    platforms_json=$(cat "${DATA_DIR}/ai-platforms.json")

    local repo_count
    repo_count=$(echo "${repositories}" | jq 'length')

    if [[ ${repo_count} -eq 0 ]]; then
        print_info "No repositories configured"
        return 0
    fi

    # Create temp directory for downloads
    local temp_base
    temp_base=$(mktemp -d "/tmp/hm-ai-tools.XXXXXX")

    print_info "Downloading from ${repo_count} repositories...\n"

    # Iterate repositories
    for ((i=0; i<repo_count; i++)); do
        local repo_name repo_url repo_branch

        repo_name=$(echo "${repositories}" | jq -r ".[$i].name")
        repo_url=$(echo "${repositories}" | jq -r ".[$i].url")
        repo_branch=$(echo "${repositories}" | jq -r ".[$i].branch // \"main\"")

        print_info "Processing repository: ${repo_name}\n"

        # Download repository
        local repo_dir="${temp_base}/${repo_name}"
        if ! download_repository "${repo_url}" "${repo_branch}" "${repo_dir}"; then
            print_warning "Failed to download ${repo_name}, skipping..."
            continue
        fi

        # Validate structure
        if ! validate_repository_structure "${repo_dir}"; then
            print_warning "Invalid repository structure in ${repo_name}, skipping..."
            continue
        fi

        # Install for each platform + resource combination
        IFS=',' read -ra platform_array <<< "${platforms}"
        IFS=',' read -ra resource_array <<< "${resources}"

        for platform in "${platform_array[@]}"; do
            for resource in "${resource_array[@]}"; do
                local dir_key="${resource}_dir"
                local target_dir

                target_dir=$(echo "${platforms_json}" | jq -r --arg p "${platform}" --arg k "${dir_key}" '.platforms[$p][$k] // empty')

                if [[ -z "${target_dir}" ]]; then
                    continue
                fi

                print_info "Installing ${resource} for ${platform}...\n"

                # Install with type filtering
                install_filtered "${repo_dir}" "${resource}" "${target_dir}" "${types}" "false" || {
                    print_warning "Failed to install ${resource} for ${platform}\n"
                }
            done
        done
    done

    # Clean up temp directory
    rm -rf "${temp_base}"

    print_info "Download and installation complete\n"
    return 0
}

#
# Main execution
#
main() {
    # Parse options
    parse_options "$@"

    # Check if non-interactive mode (any option provided)
    local interactive=true
    if [[ -n "${OPT_PLATFORMS}" ]] || [[ -n "${OPT_TYPES}" ]] || [[ -n "${OPT_RESOURCES}" ]] || [[ -n "${OPT_REPOSITORY}" ]]; then
        interactive=false
    fi

    # Load existing configuration (if any)
    local existing_config
    existing_config=$(load_ai_properties) || {
        print_error "Failed to load existing configuration"
        exit 1
    }

    # Build or wizard configuration
    local config
    if [[ "${interactive}" == "true" ]]; then
        config=$(run_wizard "${existing_config}") || {
            print_error "Wizard failed"
            exit 1
        }
    else
        config=$(build_config_from_options) || {
            print_error "Failed to build configuration"
            exit 1
        }
    fi

    # Validate configuration (handle empty arrays properly)
    local platforms types resources
    platforms=$(echo "${config}" | jq -r '(.platforms // []) | join(",")')
    types=$(echo "${config}" | jq -r '(.types // []) | join(",")')
    resources=$(echo "${config}" | jq -r '(.resources // []) | join(",")')

    if [[ -z "${platforms}" ]] || [[ -z "${resources}" ]]; then
        print_error "Configuration incomplete: platforms and resources are required"
        exit 1
    fi

    # Save configuration
    print_info "Saving configuration...\n"
    if ! save_ai_properties "${config}"; then
        print_error "Failed to save configuration"
        exit 1
    fi

    print_info "Configuration saved to config/docker/ai-properties.json\n"

    # Create platform directories
    print_info "Creating platform directories...\n"
    create_platform_directories "${platforms}" "${resources}" || {
        print_warning "Some directories could not be created"
    }

    # Resolve repositories
    print_info "Resolving repositories...\n"
    local repositories
    repositories=$(resolve_repositories "${config}") || {
        print_error "Failed to resolve repositories"
        exit 1
    }

    # Download and install
    print_info "Starting download and installation...\n"
    if ! download_and_install "${repositories}" "${platforms}" "${types}" "${resources}"; then
        print_error "Download and installation failed"
        exit 1
    fi

    echo ""
    print_info "AI tools initialization complete!\n"
    echo ""
    print_info "Configuration: config/docker/ai-properties.json\n"
    print_info "Registration: config/docker/ai-registration.json\n"
    echo ""
    print_info "You can now use AI tools with your configured platforms.\n"
    print_info "Run 'hm ai-pull' to update tools in the future.\n"
}

main "$@"
