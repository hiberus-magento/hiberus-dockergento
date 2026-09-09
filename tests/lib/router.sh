#!/usr/bin/env bash
#
# What `internal/cli/run.go` routes, read the same way by every suite that needs to know — a
# copy nobody checks is a copy that drifts, and this file exists so there is only one copy.
#
HM_ROUTER="$COMMAND_BIN_DIR/internal/cli/run.go"

#
# The command names `run.go`'s switch answers for, one per line, sorted.
#
hm_routed_commands() {
    sed -n 's/^[[:space:]]*case "\([a-z0-9_-]*\)":.*/\1/p' "$HM_ROUTER" | grep -v '^_' | sort
}
