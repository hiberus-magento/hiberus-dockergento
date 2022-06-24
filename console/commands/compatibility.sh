#!/usr/bin/env bash
set -euo pipefail

DIR=$(dirname -- "$(readlink -f -- "$0")")

# Check if command "jq" exists
if ! command -v jq &>/dev/null; then
    echo "Required 'jq' not found"
    exit
fi

# Check compatible versions
VERSIONS=$(jq -c 'keys[]' $DIR/../../data/equivalent_versions.json | tr '"' " ")
echo -e "$VERSIONS"
