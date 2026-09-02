#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# The web interface is the Go binary's, and this exists so that the command is one command.
#
# `hm` is normally the binary, which answers this without ever reaching here. What reaches here is
# somebody calling the shell entry point directly, or an installation where the binary could not
# be downloaded — and in that case the honest answer is that there is nothing to open, not
# "command not found".
#
binary="$COMMAND_BIN_DIR/bin/hm"

if [ ! -x "$binary" ]; then
    hm_fail "$HM_EXIT_ERROR" "binary_missing" \
        "The web interface is served by the ${COMMAND_BIN_NAME} binary, and it is not installed here" \
        "Reinstall ${COMMAND_BIN_NAME}, or build it with: cd $COMMAND_BIN_DIR && go build -o bin/hm ./cmd/hm"
fi

exec "$binary" web "$@"
