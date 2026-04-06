#!/usr/bin/env bash
#
# AI Tools Download Functions
# Handles downloading from remote repositories (tarball + git fallback)
#
# Copyright (c) Hiberus Tecnologías de la Información SL. All rights reserved.
# Licensed under the MIT License.

# Include guard
[[ -n "${__AI_DOWNLOAD_SH__:-}" ]] && return 0
readonly __AI_DOWNLOAD_SH__=1

set -euo pipefail

# Load dependencies
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../components/print_message.sh"

# Download configuration
readonly MAX_RETRIES=3
readonly RETRY_DELAY=2
readonly DOWNLOAD_TIMEOUT=60

#
# Validate repository URL
# Args: $1 = URL to validate
# Returns: 0 if valid HTTPS GitHub URL, 1 otherwise
#
validate_repository_url() {
    local url="$1"

    # Must be HTTPS
    if [[ ! "${url}" =~ ^https:// ]]; then
        print_error_line "Repository URL must use HTTPS protocol: ${url}"
        return 1
    fi

    # Basic URL format validation
    if [[ ! "${url}" =~ ^https://[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$ ]]; then
        print_warning_line "URL format looks unusual: ${url}"
    fi

    return 0
}

#
# Download repository as tarball using GitHub Archive API
# Args: $1 = repository URL (https://github.com/owner/repo),
#       $2 = branch name,
#       $3 = output file path
# Returns: 0 on success, 1 on failure
#
download_tarball() {
    local repo_url="$1"
    local branch="$2"
    local output_file="$3"

    # Validate URL
    validate_repository_url "${repo_url}" || return 1

    # Construct tarball URL
    # Transform: https://github.com/owner/repo → https://github.com/owner/repo/archive/refs/heads/BRANCH.tar.gz
    local tarball_url="${repo_url}/archive/refs/heads/${branch}.tar.gz"

    print_info_line "Downloading from ${tarball_url}..."

    # Download with retries
    local attempt=1
    while [[ ${attempt} -le ${MAX_RETRIES} ]]; do
        if curl -fsSL \
            --max-time "${DOWNLOAD_TIMEOUT}" \
            --retry 2 \
            --retry-delay 1 \
            -o "${output_file}" \
            "${tarball_url}"; then
            print_info_line "Downloaded successfully"
            return 0
        fi

        print_warning_line "Download attempt ${attempt}/${MAX_RETRIES} failed"

        if [[ ${attempt} -lt ${MAX_RETRIES} ]]; then
            print_info_line "Retrying in ${RETRY_DELAY} seconds..."
            sleep "${RETRY_DELAY}"
        fi

        ((attempt++))
    done

    print_error_line "Failed to download after ${MAX_RETRIES} attempts"
    return 1
}

#
# Clone repository using git (fallback method)
# Args: $1 = repository URL,
#       $2 = branch name,
#       $3 = output directory
# Returns: 0 on success, 1 on failure
#
download_git_clone() {
    local repo_url="$1"
    local branch="$2"
    local output_dir="$3"

    # Validate URL
    validate_repository_url "${repo_url}" || return 1

    # Check if git is available
    if ! command -v git >/dev/null 2>&1; then
        print_error_line "git command not found (fallback unavailable)"
        return 1
    fi

    print_info_line "Using git clone fallback for ${repo_url}..."

    # Clone with depth 1 (shallow clone)
    local attempt=1
    while [[ ${attempt} -le ${MAX_RETRIES} ]]; do
        if git clone \
            --depth 1 \
            --branch "${branch}" \
            --single-branch \
            "${repo_url}" \
            "${output_dir}" \
            2>/dev/null; then
            print_info_line "Cloned successfully"
            return 0
        fi

        print_warning_line "Clone attempt ${attempt}/${MAX_RETRIES} failed"

        if [[ ${attempt} -lt ${MAX_RETRIES} ]]; then
            print_info_line "Retrying in ${RETRY_DELAY} seconds..."
            sleep "${RETRY_DELAY}"

            # Clean up failed attempt
            rm -rf "${output_dir}"
        fi

        ((attempt++))
    done

    print_error_line "Failed to clone after ${MAX_RETRIES} attempts"
    return 1
}

#
# Download repository using best available method
# Tries tarball first (faster), falls back to git clone if available
# Args: $1 = repository URL,
#       $2 = branch name,
#       $3 = output directory (will be created)
# Returns: 0 on success, 1 on failure
#
download_repository() {
    local repo_url="$1"
    local branch="$2"
    local output_dir="$3"

    # Create output directory
    mkdir -p "${output_dir}" || {
        print_error_line "Failed to create output directory: ${output_dir}"
        return 1
    }

    # Try tarball download first (4-10x faster)
    local temp_tarball
    temp_tarball=$(mktemp "${output_dir}/repo.XXXXXX.tar.gz")

    if download_tarball "${repo_url}" "${branch}" "${temp_tarball}"; then
        # Extract tarball
        if tar -xzf "${temp_tarball}" -C "${output_dir}" --strip-components=1 2>/dev/null; then
            rm -f "${temp_tarball}"
            return 0
        else
            print_warning_line "Failed to extract tarball, trying git clone..."
            rm -f "${temp_tarball}"
        fi
    else
        rm -f "${temp_tarball}"
        print_warning_line "Tarball download failed, trying git clone..."
    fi

    # Fallback to git clone
    # Clean output directory first
    rm -rf "${output_dir:?}"/*

    download_git_clone "${repo_url}" "${branch}" "${output_dir}"
}
