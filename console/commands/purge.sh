#!/usr/bin/env bash
set -euo pipefail

${COMMAND_BIN_NAME} exec sh -c "cd ${MAGENTO_DIR} && rm -rf var/cache/* generated/* pub/static/* var/view_preprocessed/* var/page_cache/* var/generation/* dev/tests/integration/tmp/*"