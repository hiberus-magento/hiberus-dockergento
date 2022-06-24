#!/usr/bin/env bash
set -euo pipefail

${COMMAND_BIN_NAME} exec "${BIN_DIR}"/phpunit --config "${MAGENTO_DIR}"/dev/tests/unit/phpunit.xml.dist "$@"