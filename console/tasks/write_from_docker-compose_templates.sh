#!/usr/bin/env bash
set -euo pipefail

source "$HELPERS_DIR"/exit_codes.sh

regex=""

#
# Compose regex with requirements
#
compose_regex() {
    local services
    services=$(echo "$REQUIREMENTS" | jq -r 'keys | join(" ")')

    for index in $services; do
        value=$(echo "$REQUIREMENTS" | jq -r '.["'"$index"'"]')
        if [[ "$value" == *":"* ]]; then
            image="$value"
        else
            image="hiberusmagento/${index}:${value}"
        fi
        regex+="s|<${index}_version>|${image}|g; "
    done
}

#
# The mail catcher the project chose, resolved before substitution.
#
# The template stays a list of replacements with no logic in it: the two variable points —which
# service, and which image— are two more markers. Teaching the template to decide would mean
# turning the generator into something that evaluates conditions, which is a lot of machinery for
# one choice.
#
mail_regex() {
    local service="${MAIL_SERVICE:-mailhog}"

    case "$service" in
        mailhog | mailpit) ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "unknown_mail_service" \
                "'$service' is not a mail catcher this tool knows about" \
                "Set MAIL_SERVICE to mailhog or mailpit"
            ;;
    esac

    local image
    image=$(echo "$REQUIREMENTS" | jq -r --arg service "$service" '.[$service] // ""')

    if [ -z "$image" ]; then
        hm_fail "$HM_EXIT_PROJECT" "mail_service_unavailable" \
            "No $service image is defined for this Magento version" \
            "Set MAIL_SERVICE to mailhog in config/docker/properties.json"
    fi

    [[ "$image" != *":"* ]] && image="hiberusmagento/${service}:${image}"

    regex+="s|<mail_service>|${service}|g; "
    regex+="s|<mail_version>|${image}|g; "
}

#
# Adapt docker-compose template with requirements
#
write_docker_compose() {
    compose_regex
    mail_regex
    local composer_dir_name

    sed "$regex" "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" >"$DOCKER_COMPOSE_FILE"
    composer_dir_name=$(dirname "$DOCKER_COMPOSE_FILE_LINUX")
    mkdir -p "$composer_dir_name"
    cp "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.linux.template.yml" "$DOCKER_COMPOSE_FILE_LINUX"
    cp "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.mac.template.yml" "$DOCKER_COMPOSE_FILE_MAC"
}

write_docker_compose
