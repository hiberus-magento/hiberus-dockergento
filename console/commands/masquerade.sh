#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/masquerade.sh

custom_question "Are you sure you want to anonymise your database? [Y/n]" "$argument"
if [ -z "$REPLY" ] || [ "$REPLY" == 'Y' ] || [ "$REPLY" == 'y' ]; then
    masquerade_run
fi
