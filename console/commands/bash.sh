#!/usr/bin/env bash
set -euo pipefail

if [ "$#" != 0 ] && [ "$1" == "--root" ]; then
  ${COMMAND_BIN_NAME} exec --root bash
else
  ${COMMAND_BIN_NAME} exec bash
fi