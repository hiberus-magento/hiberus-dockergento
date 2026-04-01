#!/usr/bin/env bash
#
# AI Tools Reset Command
# Remove downloaded AI tools while preserving custom skills/agents
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

set -euo pipefail

# Load dependencies
source "${COMPONENTS_DIR}/print_message.sh"
source "${TASKS_DIR}/ai_registration.sh"

#
# Parse command-line options
# Sets global variables: OPT_CONFIRM
#
parse_options() {
    OPT_CONFIRM="false"

    for arg in "$@"; do
        case "${arg}" in
            --confirm)
                OPT_CONFIRM="true"
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
Usage: hm ai-reset [OPTIONS]

Remove downloaded AI tools while preserving custom skills/agents.

Options:
  --confirm    Skip confirmation prompt (for automation)

Prerequisites:
  - ai-registration.json must exist (created by 'hm ai-init' or 'hm ai-pull')

Behavior:
  - Reads tracked files from config/docker/ai-registration.json
  - Removes ONLY files that were downloaded by hm ai-init/ai-pull
  - Preserves custom skills/agents that you created manually
  - Clears ai-registration.json after successful deletion
  - Does NOT modify ai-properties.json (configuration is preserved)

Safety:
  - Whitelist-based approach: only registered files can be deleted
  - Shows list of files to be deleted and asks for confirmation
  - Refuses to operate if registration file is corrupted

Examples:
  hm ai-reset
  hm ai-reset --confirm

EOF
}

#
# Collect all tracked files from registration
# Returns: Array of file paths via stdout (newline-separated)
#
collect_tracked_files() {
    local registration
    registration=$(load_ai_registration) || {
        print_error "Failed to load registration file"
        return 1
    }

    # Get all skills
    local skills
    skills=$(get_tracked_files "skills") || true

    # Get all agents
    local agents
    agents=$(get_tracked_files "agents") || true

    # Combine
    {
        echo "${skills}"
        echo "${agents}"
    } | grep -v '^$' || true
}

#
# Remove tracked files and directories
# Args: $1 = newline-separated list of file paths
# Returns: Count of removed items
#
remove_tracked_files() {
    local files="$1"
    local removed_count=0

    while IFS= read -r file_path; do
        if [[ -z "${file_path}" ]]; then
            continue
        fi

        if [[ ! -e "${file_path}" ]]; then
            print_warning "File not found (already removed?): ${file_path}"
            continue
        fi

        # Remove file or directory
        if [[ -d "${file_path}" ]]; then
            rm -rf "${file_path}" && {
                print_info "Removed directory: ${file_path}"
                ((removed_count++))
            } || {
                print_error "Failed to remove directory: ${file_path}"
            }
        elif [[ -f "${file_path}" ]]; then
            rm -f "${file_path}" && {
                print_info "Removed file: ${file_path}"
                ((removed_count++))
            } || {
                print_error "Failed to remove file: ${file_path}"
            }
        fi
    done <<< "${files}"

    echo "${removed_count}"
}

#
# Count custom (non-tracked) skills/agents
# Args: $1 = directory path, $2 = resource type (skills|agents)
# Returns: Count of custom items
#
count_custom_items() {
    local dir_path="$1"
    local resource_type="$2"

    if [[ ! -d "${dir_path}" ]]; then
        echo "0"
        return 0
    fi

    local custom_count=0

    for item in "${dir_path}"/*; do
        if [[ ! -d "${item}" ]]; then
            continue
        fi

        # Check if tracked
        if ! is_tracked "${resource_type}" "${item}"; then
            ((custom_count++))
        fi
    done

    echo "${custom_count}"
}

#
# Show summary of what will be preserved
#
show_preserved_summary() {
    # Load platform definitions
    if [[ ! -f "data/ai-platforms.json" ]]; then
        return 0
    fi

    local platforms_json
    platforms_json=$(cat data/ai-platforms.json)

    # Load configuration to know which platforms are configured
    local config
    config=$(load_ai_properties) || echo "{}"

    local configured_platforms
    configured_platforms=$(echo "${config}" | jq -r '.platforms // [] | join(",")')

    if [[ -z "${configured_platforms}" ]]; then
        return 0
    fi

    local total_custom=0

    IFS=',' read -ra platform_array <<< "${configured_platforms}"

    for platform in "${platform_array[@]}"; do
        local skills_dir agents_dir

        skills_dir=$(echo "${platforms_json}" | jq -r --arg p "${platform}" '.platforms[$p].skills_dir // empty')
        agents_dir=$(echo "${platforms_json}" | jq -r --arg p "${platform}" '.platforms[$p].agents_dir // empty')

        if [[ -n "${skills_dir}" ]]; then
            local custom_skills
            custom_skills=$(count_custom_items "${skills_dir}" "skills")
            total_custom=$((total_custom + custom_skills))

            if [[ ${custom_skills} -gt 0 ]]; then
                print_info "  ${platform} custom skills: ${custom_skills}"
            fi
        fi

        if [[ -n "${agents_dir}" ]]; then
            local custom_agents
            custom_agents=$(count_custom_items "${agents_dir}" "agents")
            total_custom=$((total_custom + custom_agents))

            if [[ ${custom_agents} -gt 0 ]]; then
                print_info "  ${platform} custom agents: ${custom_agents}"
            fi
        fi
    done

    if [[ ${total_custom} -gt 0 ]]; then
        echo ""
        print_info "Total custom items that will be preserved: ${total_custom}"
    fi
}

#
# Main execution
#
main() {
    # Parse options
    parse_options "$@"

    # Load registration
    print_info "Loading registration data..."
    local registration
    registration=$(load_ai_registration) || {
        print_error "Failed to load ai-registration.json"
        print_error "Registration file may be corrupted or missing"
        print_info "If the file is corrupted, manually delete config/docker/ai-registration.json"
        exit 1
    }

    # Check if registration is empty
    local skills_count agents_count
    skills_count=$(echo "${registration}" | jq '.skills | length')
    agents_count=$(echo "${registration}" | jq '.agents | length')

    if [[ ${skills_count} -eq 0 ]] && [[ ${agents_count} -eq 0 ]]; then
        print_info "No tracked AI tools found in registration"
        print_info "Nothing to reset"
        exit 0
    fi

    # Collect tracked files
    print_info "Collecting tracked files..."
    local tracked_files
    tracked_files=$(collect_tracked_files) || {
        print_error "Failed to collect tracked files"
        exit 1
    }

    local file_count
    file_count=$(echo "${tracked_files}" | grep -c '[^[:space:]]' || echo "0")

    if [[ ${file_count} -eq 0 ]]; then
        print_info "No files to remove"
        exit 0
    fi

    # Show what will be removed
    echo ""
    print_warning "The following ${file_count} items will be removed:"
    echo ""
    echo "${tracked_files}"
    echo ""

    # Show what will be preserved
    print_info "Custom skills/agents will be preserved:"
    show_preserved_summary

    # Confirmation prompt (unless --confirm flag)
    if [[ "${OPT_CONFIRM}" != "true" ]]; then
        echo ""
        print_warning "This action cannot be undone."
        print_info "Continue with deletion? (y/N)"

        local confirmation
        read -r confirmation

        if [[ ! "${confirmation}" =~ ^[Yy] ]]; then
            print_info "Reset cancelled"
            exit 0
        fi
    fi

    # Remove tracked files
    echo ""
    print_info "Removing tracked files..."
    local removed_count
    removed_count=$(remove_tracked_files "${tracked_files}")

    # Clear registration
    print_info "Clearing registration..."
    if ! clear_registration; then
        print_error "Failed to clear registration file"
        exit 1
    fi

    # Success
    echo ""
    print_info "AI tools reset complete!"
    print_info "Removed ${removed_count} items"
    echo ""
    print_info "Configuration file preserved: config/docker/ai-properties.json"
    print_info "Registration file cleared: config/docker/ai-registration.json"
    echo ""
    print_info "To re-download AI tools, run: hm ai-pull"
}

main "$@"
