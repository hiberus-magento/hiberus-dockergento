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
    
    # Create custom properties for current project
    cat "$CUSTOM_PROPERTIES_DIR"/properties.json | jq -n \
        --arg MAGENTO_DIR "$MAGENTO_DIR" \
        --arg COMPOSE_PROJECT_NAME "$COMPOSE_PROJECT_NAME" \
        --arg DOMAIN "$DOMAIN" \
    '$ARGS.named' > "$CUSTOM_PROPERTIES_DIR"/properties.json
}