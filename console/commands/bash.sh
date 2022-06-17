#!/usr/bin/env bash
set -euo pipefail

if [ "$#" != 0 ] && [ "$1" == "--root" ];
then
   ${COMMANDS_DIR}/exec.sh --root bash
else
    ${COMMANDS_DIR}/exec.sh bash
fi