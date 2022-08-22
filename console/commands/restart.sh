#!/usr/bin/env bash
set -euo pipefail

$COMMAND_BIN_NAME stop $@
$COMMAND_BIN_NAME start $@