#!/usr/bin/env bash
set -euo pipefail

#
# Through a shell, and not as one argument. `docker compose exec` takes a command and its
# arguments, so a single argument with spaces in it is a file whose name has spaces: Docker
# reported `stat ./vendor/bin/phpunit --config ...: no such file or directory` and the exit code
# was somebody else's, so a CI running this was told the tests passed without one having run.
#
"$COMMANDS_DIR"/exec.sh sh -c "$BIN_DIR/phpunit --config ./dev/tests/unit/phpunit.xml.dist $*"