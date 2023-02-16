#!/usr/bin/env bash
set -euo pipefail

#
#
#
sanitize_mirror_path() {
    # Remove magento slash at end
    echo "${1%/}"
}
