#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

# Check if command "jq" exists
if ! command -v jq &>/dev/null; then
    print_error "Required 'jq' not found"
    print_question "https://stedolan.github.io/jq/download/"
    exit 0
fi

# Check compatible versions
versions=$(jq -r 'keys[]' < "$DATA_DIR"/equivalent_versions.json)

topBottom=$(printf '=%.0s' {1..100} )
table=""
perviousVersion=23

printf "\n"

# Compose tbody
for version in $versions; do
    versionNumber=${version:0:3}
    versionNumber=${versionNumber//./}

    if [ "$versionNumber" -gt $perviousVersion ]; then
        table="$(echo -n "$table|\n")"
        perviousVersion=$versionNumber
    fi
    table=$(printf "$table| %-9s" "${version}")
done

# Print table
title=$(printf "%63s" "SUPPORTED MAGENTO VERSIONS")
print_table "$topBottom"
print_table "$title"
print_table "$topBottom"
print_table "$table|"
print_table "$topBottom\n"