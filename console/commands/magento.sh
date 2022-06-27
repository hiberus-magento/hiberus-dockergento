#!/usr/bin/env bash
set -euo pipefail

"$COMMAND_BIN_NAME" exec php "$MAGENTO_DIR"/bin/magento "$@"