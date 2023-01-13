#!/usr/bin/env bash
set -euo pipefail

if [ "$#" != 0 ] && [ "$1" == "-r" ]; then
    "$COMMANDS_DIR"/exec.sh -r bash
else
   "$COMMANDS_DIR"/exec.sh bash
fi
