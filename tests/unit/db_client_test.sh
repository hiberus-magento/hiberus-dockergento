#!/usr/bin/env bash
#
# The connection string, and which client gets opened how.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/db_client.sh"

#
# The compose configuration as `docker compose config --format json` resolves it. Stubbed here so
# the shape is fixed: what is being tested is that nothing is guessed.
#
compose_config_json() { printf '%s' "$CONFIG"; }

CONFIG='{"services":{"db":{"image":"mariadb:10.6","ports":[{"target":3306,"published":"3307"}],
  "environment":{"MYSQL_USER":"tienda","MYSQL_PASSWORD":"secreta","MYSQL_DATABASE":"tienda_db"}}}}'

# ---------------------------------------------------------------- the connection

test_case "the details come from the project, not from a default"
hm_db_connection ""
assert_equals "3307" "$HM_DB_PORT"
assert_equals "tienda" "$HM_DB_USER"
assert_equals "tienda_db" "$HM_DB_NAME"

test_case "and the string is one a client understands"
assert_equals "mysql://tienda:secreta@127.0.0.1:3307/tienda_db" "$(hm_db_url)"

test_case "a project with no credentials of its own gets the tool's"
CONFIG='{"services":{"db":{"ports":[{"target":3306,"published":"3306"}]}}}'
hm_db_connection ""
assert_equals "magento" "$HM_DB_USER"
assert_equals "magento" "$HM_DB_NAME"

# ---------------------------------------------------------------- no published port
#
# What the global proxy does to the database: MySQL carries no hostname, Traefik cannot route it,
# so the overlay removes the published port. Opening a client at 127.0.0.1:3306 there would
# connect it to whatever else is listening.

test_case "a database with no published port is not a connection"
CONFIG='{"services":{"db":{"ports":[]}}}'
hm_db_connection "" && r=conecta || r=no
assert_equals "no" "$r"

test_case "unless a tunnel is already holding one open"
hm_db_connection "13306"
assert_equals "13306" "$HM_DB_PORT"
assert_contains "$(hm_db_url)" ":13306/"

test_case "a port on another target does not count as the database's"
CONFIG='{"services":{"db":{"ports":[{"target":33060,"published":"33060"}]}}}'
hm_db_connection "" && r=conecta || r=no
assert_equals "no" "$r"

# ---------------------------------------------------------------- the clients

test_case "each client is opened the way that client is opened"
assert_contains "$(MACHINE=mac hm_db_client_command tableplus 'mysql://x')" "open -a TablePlus"
assert_contains "$(MACHINE=mac hm_db_client_command sequelace 'mysql://x')" "Sequel Ace"
assert_contains "$(MACHINE=linux hm_db_client_command dbeaver 'mysql://x')" "dbeaver"

test_case "and on Linux anything else is handed to the desktop"
assert_contains "$(MACHINE=linux hm_db_client_command tableplus 'mysql://x')" "xdg-open"

test_case "the names are the ones people read, not the command names"
assert_equals "Sequel Ace" "$(hm_db_client_name sequelace)"
assert_equals "TablePlus" "$(hm_db_client_name tableplus)"

# ---------------------------------------------------------------- the three commands

test_case "the three launchers are the same command with a different word"
for client in tableplus sequelace dbeaver; do
    assert_contains "$(cat "$COMMANDS_DIR/$client.sh")" "hm_db_client_run \"$client\""
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
