#!/usr/bin/env bash
set -euo pipefail

$COMMAND_BIN_NAME exec sh -c "cd $MAGENTO_DIR/dev/tests/integration && ${WORKDIR_PHP}/${BIN_DIR}/phpunit --config phpunit.xml" "$@"