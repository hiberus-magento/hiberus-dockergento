#!/usr/bin/env bash

#
# Check if version is greater than or equal to target (semver comparison)
# Usage: version_gte "2.4.6" "2.4.5" && echo "yes"
#
version_gte() {
    local version="$1"
    local target="$2"
    [ "$(printf '%s\n' "$target" "$version" | sort -V | head -n1)" == "$target" ]
}
