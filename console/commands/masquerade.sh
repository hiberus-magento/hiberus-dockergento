#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/masquerade.sh
source "$TASKS_DIR"/anonymisation.sh

if is_non_interactive; then
    # --yes answers this command's own confirmation prompt
    confirmation="y"
else
    read -p "Are you sure you want to anonymise your database? [Y/n]: " confirmation
fi

if [ -z "$confirmation" ] || [ "$confirmation" == 'Y' ] || [ "$confirmation" == 'y' ]; then
    if masquerade_run; then
        # Recorded so that everything else can stop guessing: the context an agent reads, the
        # doctor, and describe
        hm_anonymisation_record
    fi
fi
