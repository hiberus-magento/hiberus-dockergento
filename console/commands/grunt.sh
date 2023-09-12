#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

# Check if PHP container is running
is_run_service "phpfpm"

# Check command usage
if [ "$#" -ne 2 ] || [[ "$1" != */* ]]; then
    print_error "Error: You should specify a Magento theme and a locale.\n\n"
    print_info  "  - Usage: "
    print_code "$COMMAND_BIN_NAME grunt <Vendor/theme> <locale_LOCALE>\n\n"
    exit
fi

grunt_theme=$1
grunt_locale=$2

# Rename files
print_processing "Preparing config files...\n"
docker-compose exec phpfpm bash -c "if [ -f \"package.json.sample\" ] && [ ! -f \"package.json\" ]; then cp package.json.sample package.json; fi"
docker-compose exec phpfpm bash -c "if [ -f \"grunt-config.json.sample\" ] && [ ! -f \"grunt-config.json\" ]; then cp grunt-config.json.sample grunt-config.json; fi"
docker-compose exec phpfpm bash -c "if [ -f \"Gruntfile.js.sample\" ] && [ ! -f \"Gruntfile.js\" ]; then cp Gruntfile.js.sample Gruntfile.js; fi"

# NPM Install
docker-compose exec phpfpm bash -c "npm install && npm update"

# Prepare local-themes file
docker-compose exec phpfpm bash -c \
    "if [ -d \"dev/tools/grunt/configs\" ]; then echo \"module.exports = {magento: {area: 'frontend', name: '${grunt_theme}', locale: '${grunt_locale}', files: ['css/styles-m', 'css/styles-l'], dsl: 'less' }};\" > dev/tools/grunt/configs/local-themes.js; fi"
print_processing "Prepared grunt configurations...\n"

# Launch grunt
print_processing "Compiling styles...\n"
docker-compose exec phpfpm bash -c "grunt exec:magento && grunt watch:magento"
