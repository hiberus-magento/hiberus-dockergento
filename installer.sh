#!/usr/bin/env bash
#
# The installer's old name, kept because 1.x documentation and bookmarks point at it:
#
#   curl https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/installer.sh | bash
#
# It only runs install.sh, which is the installer, with the same arguments. Piped from curl there is
# no file beside this one to run, so install.sh is fetched from the same branch (HM_INSTALLER_REF,
# main by default). Run from a checkout, the install.sh next to it is used.
#
# The whole body is one function called on the last line: a download cut short runs nothing.
#
set -euo pipefail

run_installer() {
    local here="" script ref="${HM_INSTALLER_REF:-main}"

    if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
        here="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi

    if [ -n "$here" ] && [ -f "$here/install.sh" ]; then
        exec bash "$here/install.sh" "$@"
    fi

    script="$(curl -fsSL "https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/$ref/install.sh")" || {
        echo "Could not download install.sh from the $ref branch. Nothing was installed." >&2
        exit 1
    }

    bash -c "$script" install.sh "$@"
}

run_installer "$@"
