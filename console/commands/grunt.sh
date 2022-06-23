#!/usr/bin/env bash
set -euo pipefail

# Check command usage
if [ "$#" -ne 2 ] || [[ "$1" != */* ]]; then
    printf "${RED}Error: You should specify a Magento theme and a locale.\n- Usage: ${COMMAND_BIN_NAME} grunt <Vendor/theme> <locale_LOCALE>${COLOR_RESET}\n"
    exit
fi

GRUNT_THEME=$1
GRUNT_LOCALE=$2

# Check if PHP container is running
if [ -z "$(docker ps | grep phpfpm)" ]; then
    printf "${RED}Error: PHP container is not running!${COLOR_RESET}\n"
    exit
fi

# Rename files
printf "Preparing config files...\n"
docker-compose exec phpfpm bash -c "if [ -f \"package.json.sample\" ] && [ ! -f \"package.json\" ]; then cp package.json.sample package.json; fi"
docker-compose exec phpfpm bash -c "if [ -f \"grunt-config.json.sample\" ] && [ ! -f \"grunt-config.json\" ]; then cp grunt-config.json.sample grunt-config.json; fi"
docker-compose exec phpfpm bash -c "if [ -f \"Gruntfile.js.sample\" ] && [ ! -f \"Gruntfile.js\" ]; then cp Gruntfile.js.sample Gruntfile.js; fi"

# NPM Install
docker-compose exec phpfpm bash -c "npm install && npm update"

# Prepare local-themes file
docker-compose exec phpfpm bash -c \
    "if [ -d \"dev/tools/grunt/configs\" ]; then echo \"module.exports = {magento: {area: 'frontend', name: '${GRUNT_THEME}', locale: '${GRUNT_LOCALE}', files: ['css/styles-m', 'css/styles-l'], dsl: 'less' }};\" > dev/tools/grunt/configs/local-themes.js; fi"
printf "Prepared grunt configurations...\n"

# Launch grunt
printf "${GREEN}Compiling styles...\n${COLOR_RESET}"
docker-compose exec phpfpm bash -c "grunt exec:magento && grunt watch:magento"
