#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/masquerade.sh

if is_non_interactive; then
    # --yes answers this command's own confirmation prompt
    confirmation="y"
else
    read -p "Are you sure you want to anonymise your database? [Y/n]: " confirmation
fi

if [ -z "$confirmation" ] || [ "$confirmation" == 'Y' ] || [ "$confirmation" == 'y' ]; then
    masquerade_run
fi
