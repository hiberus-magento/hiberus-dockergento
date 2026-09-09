#!/usr/bin/env bash

#
# Runs the ${COMMAND_BIN_NAME} binary in place of a shell twin that has been ported and deleted.
#
# It runs, it does not `exec`: a caller like `composer.sh` has work left to do after the copy
# (`:52-63`), and `exec` would end the script at the first one.
#
declare -F hm_fail >/dev/null 2>&1 || source "$HELPERS_DIR"/exit_codes.sh

hm_run_binary() {
    local binary="$COMMAND_BIN_DIR/bin/hm"

    if [ ! -x "$binary" ]; then
        hm_fail "$HM_EXIT_ERROR" "binary_missing" \
            "$1 needs the ${COMMAND_BIN_NAME} binary, and it is not installed here" \
            "Reinstall ${COMMAND_BIN_NAME}, or build it with: cd $COMMAND_BIN_DIR && go build -o bin/hm ./cmd/hm"
    fi

    "$binary" "$@"
}
