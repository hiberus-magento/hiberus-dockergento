#!/usr/bin/env bash
set -euo pipefail

#
# Set properties in <root_project>/<docker_config>/properties
#
save_properties() {
    print_info "Saving custom properties file: '$DOCKER_CONFIG_DIR/properties'\n"

    # Create directory if not extis
    mkdir -p $CUSTOM_PROPERTIES_DIR

    # Create file if not exits
    if [[ ! -f "$CUSTOM_PROPERTIES_DIR"/properties.json ]];then
        echo "{}" > "$CUSTOM_PROPERTIES_DIR"/properties.json
    fi
    
    if [ -z ${DOMAIN+x} ]; then
        get_domain
    fi
    
    source "$HELPERS_DIR"/project_name.sh

    local properties_file="$CUSTOM_PROPERTIES_DIR/properties.json"
    local derived had_name
    derived=$(hm_derive_project_name "${HM_ROOT:-$PWD}")
    had_name=$(jq -r 'has("COMPOSE_PROJECT_NAME") and (.COMPOSE_PROJECT_NAME != "")' "$properties_file")

    #
    # The name is recorded only when it is a decision.
    #
    # This file is committed, so whatever it says travels to every clone of the project. Writing
    # the name that the directory would have given anyway is what made a second clone inherit the
    # first one's identity — same containers, same volumes, neither of them asked for.
    #
    # A project that already had a name keeps it: removing it would be renaming somebody's
    # environment, and its volumes do not follow.
    #
    # The rest of the file is merged rather than replaced. It used to be rebuilt from three keys,
    # so any other property a project had —a custom BIN_DIR, for instance— disappeared on the
    # next `setup`.
    #
    jq \
        --arg magento_dir "$MAGENTO_DIR" \
        --arg project_name "$COMPOSE_PROJECT_NAME" \
        --arg domain "$DOMAIN" \
        --arg derived "$derived" \
        --arg mail_service "${MAIL_SERVICE:-mailhog}" \
        --argjson had_name "$had_name" \
        '. + {MAGENTO_DIR: $magento_dir, DOMAIN: $domain}
         | if $had_name or ($project_name != $derived and $project_name != "")
           then . + {COMPOSE_PROJECT_NAME: $project_name}
           else del(.COMPOSE_PROJECT_NAME) end
         | if $mail_service == "mailhog"
           then del(.MAIL_SERVICE)
           else . + {MAIL_SERVICE: $mail_service} end' \
        "$properties_file" > "$properties_file.tmp" && mv "$properties_file.tmp" "$properties_file"
}