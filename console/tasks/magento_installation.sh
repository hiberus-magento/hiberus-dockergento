#!/usr/bin/env bash
set -euo pipefail 

# Magento installation
$COMMAND_BIN_NAME composer install
$COMMAND_BIN_NAME install "$DOMAIN"

# Magento commands
$COMMAND_BIN_NAME magento setup:upgrade
$COMMAND_BIN_NAME magento deploy:mode:set developer

# Domain and certificate
$COMMAND_BIN_NAME ssl "$DOMAIN"
$COMMAND_BIN_NAME set-host "$DOMAIN" --no-database

$COMMAND_BIN_NAME restart