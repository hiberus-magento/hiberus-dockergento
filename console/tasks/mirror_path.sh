#!/usr/bin/env bash
set -euo pipefail

remove_magento_dir_prefix() {
    path_mirror=$1
    echo "${path_mirror#"$MAGENTO_DIR/"}"
}

remove_magento_slash_at_end() {
    path_mirror=$1
    echo "${path_mirror%/}"
}

add_magento_dir_prefix() {
    path_mirror=$1
    echo "$MAGENTO_DIR/$path_mirror"
}

sanitize_mirror_path() {
    path_mirror=$1

    path_mirror=$(remove_magento_dir_prefix "$path_mirror")
    path_mirror=$(remove_magento_slash_at_end "$path_mirror")
    path_mirror=$(add_magento_dir_prefix "$path_mirror")

    echo "$path_mirror"
}
