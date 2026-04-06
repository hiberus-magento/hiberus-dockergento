#!/usr/bin/env bash
#
# AI Tools Configuration Wizard
# Interactive prompts for configuring AI platforms, skill types, and resources
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

# Include guard
[[ -n "${__AI_WIZARD_SH__:-}" ]] && return 0
readonly __AI_WIZARD_SH__=1

set -euo pipefail

# Load dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../components/print_message.sh"
source "${SCRIPT_DIR}/../components/input_info.sh"

#
# Display multi-select menu with checkboxes
# Args: $1 = prompt text,
#       $2 = JSON object with options (key = id, value = {name, description}),
#       $3 = pre-selected items (comma-separated, optional)
# Returns: Comma-separated list of selected IDs via stdout
#
multi_select_menu() {
    local prompt="$1"
    local options_json="$2"
    local preselected="${3:-}"

    echo "" >&2
    print_info "${prompt}" >&2
    echo "" >&2

    # Parse options into arrays
    local -a option_ids=()
    local -a option_names=()
    local -a option_descriptions=()

    while IFS= read -r line; do
        local id name desc
        id=$(echo "${line}" | jq -r '.id')
        name=$(echo "${line}" | jq -r '.name')
        desc=$(echo "${line}" | jq -r '.description')

        option_ids+=("${id}")
        option_names+=("${name}")
        option_descriptions+=("${desc}")
    done < <(echo "${options_json}" | jq -c 'to_entries[] | {id: .key, name: .value.name, description: .value.description}')

    # Build preselected list (bash 3.2 compatible - no associative arrays)
    local preselected_list=",${preselected},"

    # Display options with numbers
    for i in "${!option_ids[@]}"; do
        local checkbox="[ ]"
        local num=$((i + 1))
        # Check if option_id is in preselected list
        if [[ "${preselected_list}" == *",${option_ids[$i]},"* ]]; then
            checkbox="[X]"
        fi

        printf "  %s %d) %s - %s\n" "${checkbox}" "${num}" "${option_names[$i]}" "${option_descriptions[$i]}" >&2
    done

    echo "" >&2
    printf "> " >&2

    # Read user input
    local user_input
    read -r user_input

    # Handle 'all' selection
    if [[ "${user_input}" == "all" ]]; then
        local all_ids
        all_ids=$(IFS=','; echo "${option_ids[*]}")
        echo "${all_ids}"
        return 0
    fi

    # Handle empty input
    if [[ -z "${user_input}" ]]; then
        # Keep preselected or return empty if nothing selected
        echo "${preselected}"
        return 0
    fi

    # Parse comma-separated input
    IFS=',' read -ra selections <<< "${user_input}"
    local -a result=()

    # Process selections only if array is not empty
    if [[ ${#selections[@]} -gt 0 ]]; then
        for sel in "${selections[@]}"; do
            # Trim whitespace
            sel=$(echo "${sel}" | xargs)

            # Skip empty selections
            [[ -z "${sel}" ]] && continue

            # Check if numeric index
            if [[ "${sel}" =~ ^[0-9]+$ ]]; then
                local idx=$((sel - 1))
                if [[ ${idx} -ge 0 ]] && [[ ${idx} -lt ${#option_ids[@]} ]]; then
                    result+=("${option_ids[$idx]}")
                fi
            else
                # Assume it's an ID
                for opt_id in "${option_ids[@]}"; do
                    if [[ "${opt_id}" == "${sel}" ]]; then
                        result+=("${opt_id}")
                        break
                    fi
                done
            fi
        done
    fi

    # Return comma-separated result
    local output
    output=$(IFS=','; echo "${result[*]}")
    echo "${output}"
}

#
# Wizard: Select resource types (skills, agents, or both)
# Args: $1 = pre-selected resources (comma-separated, optional)
# Returns: Comma-separated list: "skills", "agents", or "skills,agents"
#
wizard_select_resources() {
    local preselected="${1:-}"

    local resources_json
    resources_json=$(cat <<'EOF'
{
  "skills": {
    "name": "Skills",
    "description": "AI assistant skills for common development tasks"
  },
  "agents": {
    "name": "Agents",
    "description": "Autonomous agents for complex workflows"
  }
}
EOF
)

    multi_select_menu "Which resources would you like to manage?" "${resources_json}" "${preselected}"
}

#
# Wizard: Select AI platforms
# Args: $1 = pre-selected platforms (comma-separated, optional)
# Returns: Comma-separated list of platform IDs
#
wizard_select_platforms() {
    local preselected="${1:-}"

    # Load platform definitions
    local platforms_file="${DATA_DIR:-data}/ai-platforms.json"
    if [[ ! -f "${platforms_file}" ]]; then
        print_error "Platform definitions not found: ${platforms_file}" >&2
        echo "" >&2
        return 1
    fi

    local platforms_json
    platforms_json=$(jq '.platforms' "${platforms_file}")

    multi_select_menu "Which AI platforms do you use?" "${platforms_json}" "${preselected}"
}

#
# Wizard: Select skill types
# Args: $1 = pre-selected types (comma-separated, optional)
# Returns: Comma-separated list of type IDs
#
wizard_select_skill_types() {
    local preselected="${1:-}"

    # Load skill type definitions
    local skill_types_file="${DATA_DIR:-data}/ai-skill-types.json"
    if [[ ! -f "${skill_types_file}" ]]; then
        print_error "Skill type definitions not found: ${skill_types_file}" >&2
        echo "" >&2
        return 1
    fi

    local types_json
    types_json=$(jq '.skill_types' "${skill_types_file}")

    multi_select_menu "Which skill types does your project need?" "${types_json}" "${preselected}"
}

#
# Wizard: Optional custom repository configuration
# Args: $1 = existing custom repositories JSON array (optional)
# Returns: JSON array of custom repositories via stdout
#
wizard_custom_repositories() {
    local existing_repos="${1:-[]}"

    echo "" >&2
    print_info "Would you like to add a custom repository? (y/N)" >&2
    echo "" >&2
    local add_custom
    read -r add_custom

    if [[ ! "${add_custom}" =~ ^[Yy] ]]; then
        echo "${existing_repos}"
        return 0
    fi

    # Collect repository URL
    echo "" >&2
    print_info "Enter repository URL (must be HTTPS):" >&2
    echo "" >&2
    local repo_url
    read -r repo_url

    # Validate HTTPS
    if [[ ! "${repo_url}" =~ ^https:// ]]; then
        print_error "Repository URL must use HTTPS protocol" >&2
        echo "" >&2
        echo "${existing_repos}"
        return 0
    fi

    # Collect branch name
    echo "" >&2
    print_info "Enter branch name (default: main):" >&2
    echo "" >&2
    local branch
    read -r branch
    branch="${branch:-main}"

    # Collect repository name (optional)
    echo "" >&2
    print_info "Enter repository name (optional, for display):" >&2
    echo "" >&2
    local repo_name
    read -r repo_name

    if [[ -z "${repo_name}" ]]; then
        # Extract name from URL
        repo_name=$(basename "${repo_url}" .git)
    fi

    # Add to repositories array
    local updated_repos
    updated_repos=$(echo "${existing_repos}" | jq \
        --arg name "${repo_name}" \
        --arg url "${repo_url}" \
        --arg branch "${branch}" \
        '. += [{name: $name, url: $url, branch: $branch, types: []}]')

    echo "${updated_repos}"
}

#
# Run full configuration wizard
# Args: $1 = existing config JSON (optional, for reconfiguration)
# Returns: Complete configuration JSON via stdout
#
run_wizard() {
    local existing_config="$1"
    if [[ -z "${existing_config}" ]]; then
        existing_config="{}"
    fi

    echo "" >&2
    print_header "AI Tools Configuration Wizard" >&2
    echo "" >&2
    print_info "Instructions: Enter numbers (1,2), IDs (claude,cursor), 'all', or press Enter to keep current" >&2
    echo "" >&2

    # Extract existing values (handle empty arrays properly)
    local existing_resources
    existing_resources=$(echo "${existing_config}" | jq -r '(.resources // []) | join(",")' 2>/dev/null || echo "")

    local existing_platforms
    existing_platforms=$(echo "${existing_config}" | jq -r '(.platforms // []) | join(",")' 2>/dev/null || echo "")

    local existing_types
    existing_types=$(echo "${existing_config}" | jq -r '(.types // []) | join(",")' 2>/dev/null || echo "")

    local existing_custom_repos
    existing_custom_repos=$(echo "${existing_config}" | jq -c '.custom_repositories // []' 2>/dev/null || echo "[]")

    # Step 1: Resource selection
    local selected_resources
    selected_resources=$(wizard_select_resources "${existing_resources}")

    # Step 2: Platform selection
    local selected_platforms
    selected_platforms=$(wizard_select_platforms "${existing_platforms}")

    # Step 3: Skill type selection
    local selected_types
    selected_types=$(wizard_select_skill_types "${existing_types}")

    # Step 4: Custom repositories (optional)
    local custom_repos
    custom_repos=$(wizard_custom_repositories "${existing_custom_repos}")

    # Build final configuration JSON
    local config_json
    config_json=$(jq -n \
        --arg resources "${selected_resources}" \
        --arg platforms "${selected_platforms}" \
        --arg types "${selected_types}" \
        --argjson custom_repos "${custom_repos}" \
        '{
            resources: ($resources | split(",") | map(select(length > 0))),
            platforms: ($platforms | split(",") | map(select(length > 0))),
            types: ($types | split(",") | map(select(length > 0))),
            custom_repositories: $custom_repos,
            configured_at: (now | strftime("%Y-%m-%d %H:%M:%S"))
        }')

    echo "${config_json}"
}
