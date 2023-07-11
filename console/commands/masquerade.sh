#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/masquerade.sh

read -p "Are you sure you want to anonymise your database? [Y/n]: " confirmation
if [ -z "$confirmation" ] || [ "$confirmation" == 'Y' ] || [ "$confirmation" == 'y' ]; then
    masquerade_run
fi
