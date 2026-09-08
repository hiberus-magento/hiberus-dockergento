#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# A Compose subcommand this tool does not implement itself is the Go binary's now. This exists so
# the command is one command: `hm` is normally the binary, which answers this without ever
# reaching here; what reaches here is somebody calling the shell entry point directly.
#
binary="$COMMAND_BIN_DIR/bin/hm"

if [ ! -x "$binary" ]; then
    hm_fail "$HM_EXIT_ERROR" "binary_missing" \
        "docker-compose needs the ${COMMAND_BIN_NAME} binary, and it is not installed here" \
        "Reinstall ${COMMAND_BIN_NAME}, or build it with: cd $COMMAND_BIN_DIR && go build -o bin/hm ./cmd/hm"
fi

exec "$binary" docker-compose "$@"
