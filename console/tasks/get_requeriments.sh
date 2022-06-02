#!/usr/bin/env bash
set -euo pipefail

#
# Print current requeriments
# 
print_requeriments() {
    echo -e "\n${CYAN} ------------------------------- ${COLOR_RESET}"
    echo -e "${CYAN} VERSIONS ${COLOR_RESET}"
    echo -e "${CYAN} ------------------------------- ${COLOR_RESET}"
    echo -e "${CYAN} php:${COLOR_RESET} ${php_requeriment}"
    echo -e "${CYAN} mysql:${COLOR_RESET} ${mysql_requeriment}"
    echo -e "${CYAN} elasticserach:${COLOR_RESET} ${elastic_requeriment}"
    echo -e "${CYAN} redis:${COLOR_RESET} ${redis_requeriment}"
    echo -e "${CYAN} varnish:${COLOR_RESET} ${varnish_requeriment}"
    echo -e "${CYAN} composer:${COLOR_RESET} ${composer_requeriment}"
    echo -e "${CYAN} ------------------------------- ${COLOR_RESET}\n"
}

#
# Ask if user wants to change requeriments
# 
change_requeriments() {
    print_requeriments
    while true; do
        printf "${BLUE} Do you want edit any version? ${COLOR_RESET}";
        read yn
        case $yn in
            [Yy]* ) edit_versions; break;;
            [Nn]* ) exit;;
            * ) echo "Please answer yes or no.";;
        esac
    done
}

#
# Changes especific service value
# 
edit_version() {
    SERVICE_NAME=$1
    OPTIONS=$2
    printf "${BLUE}${SERVICE_NAME} version:\n${COLOR_RESET}"

    select SELECT_RESULT in ${OPTIONS};
    do
        if $(${TASKS_DIR}/in_list.sh "${SELECT_RESULT}" "${OPTIONS}"); then
            break
        fi
        if $(${TASKS_DIR}/in_list.sh "${REPLY}" "${OPTIONS}"); then
            PHP_VERSION_SELECT=${REPLY}
            break
        fi
        echo "invalid option '${REPLY}'"
    done

    if [[ ${SELECT_RESULT} != '' ]];
    then
     case $SERVICE_NAME in
        Php)
            php_requeriment=$SELECT_RESULT
            ;;
        Mysql)
            mysql_requeriment=$SELECT_RESULT
            ;;
        Elasticserach)
            elastic_requeriment=$SELECT_RESULT
            ;;
        Redis)
            redis_requeriment=$SELECT_RESULT
            ;;
        Varnish)
            varnish_requeriment=$SELECT_RESULT
            ;;
        Composer)
            composer_requeriment=$SELECT_RESULT
            ;;
        esac
    fi
}

#
# Select editable services and changes her value
# 
edit_versions() {
    printf "${BLUE}Choose service:\n${COLOR_RESET}"
    OPTIONS="Php Mysql Elasticserach Redis Varnish Composer"
    select SELECT_RESULT in ${OPTIONS};
    do
        if $(${TASKS_DIR}/in_list.sh "${SELECT_RESULT}" "${OPTIONS}"); then
            break
        fi
        if $(${TASKS_DIR}/in_list.sh "${REPLY}" "${OPTIONS}"); then
            PHP_VERSION_SELECT=${REPLY}
            break
        fi
        echo "invalid option '${REPLY}'"
    done

    if [[ ${SELECT_RESULT} != '' ]];
    then
     case $SELECT_RESULT in
        Php)
            edit_version "Php" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .php] | unique  | join(" ")')"  
            ;;
        Mysql)
            edit_version "Mysql" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .mysql] | unique  | join(" ")')"  
            ;;
        Elasticserach)
            edit_version "Elasticserach" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .elastic] | unique  | join(" ")')"  
            ;;
        Redis)
            edit_version "Redis" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .redis] | unique  | join(" ")')"  
            ;;
        Varnish)
            edit_version "Varnish" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .varnish] | unique  | join(" ")')"  
            ;;
        Composer)
            edit_version "Composer" "$(cat "${DATA_DIR}/requeriments.json" | jq -r '[.[] | .composer] | unique  | join(" ")')"  
            ;;
        esac
    fi

    change_requeriments
}

set_requeriments() {
    export PHP_VERSION="${php_requeriment}"
    export MYSQL_VERSION="${mysql_requeriment}"
    export ELASTIC_VERSION="${elastic_requeriment}"
    export REDIS_VERSION="${redis_requeriment}"
    export VARNISH_VERSION="${varnish_requeriment}"
    export COMPOSER_VERSION="${composer_requeriment}"
}

# Check if command "jq" exists
if ! command -v jq  &> /dev/null
then
    printf "${RED}Required 'jq' not found${COLOR_RESET}"
    printf "${BLUE}https://stedolan.github.io/jq/download/${COLOR_RESET}"
    exit
fi

# Get all version requeriments
REQUERIMENTS=$(cat "${DATA_DIR}/requeriments.json" | jq -r '.['\"$1\"']')

# Save requeriments
php_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.php')
mysql_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.mysql')
elastic_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.elastic')
redis_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.redis')
varnish_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.varnish')
composer_requeriment=$(echo "${REQUERIMENTS}" | jq -r '.composer');

change_requeriments

set_requeriments
