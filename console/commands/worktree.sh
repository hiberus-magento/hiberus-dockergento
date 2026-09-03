#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# Branch environments are the Go binary's, all three subcommands of them.
#
# This exists so that the command is one command. `hm` is normally the binary, which answers this
# without ever reaching here; what reaches here is somebody calling the shell entry point directly.
#
# It is a delegation and not a second implementation on purpose. While both existed they wrote the
# same registrations, which is what made porting them one at a time safe — and it is also what
# stopped the registry from becoming anything better than a directory of small files. Two writers
# and one of them reading somewhere else is a branch environment created that nothing else can see.
#
binary="$COMMAND_BIN_DIR/bin/hm"

if [ ! -x "$binary" ]; then
    hm_fail "$HM_EXIT_ERROR" "binary_missing" \
        "Branch environments need the ${COMMAND_BIN_NAME} binary, and it is not installed here" \
        "Reinstall ${COMMAND_BIN_NAME}, or build it with: cd $COMMAND_BIN_DIR && go build -o bin/hm ./cmd/hm"
fi

exec "$binary" worktree "$@"
