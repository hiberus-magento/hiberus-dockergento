#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/properties.sh
source "$HELPERS_DIR"/domain_resolution.sh
source "$HELPERS_DIR"/exit_codes.sh

modify_database=true
remove_entry=false

#
# The marker is the whole point of it. Entries were appended and never removed, and there was
# nothing in the line to say who put it there — so they accumulate for as long as the machine
# lives and nobody dares delete one. With a marker the tool can find its own, and leave alone
# anything a person wrote.
#
HM_HOSTS_MARKER="# added by $COMMAND_BIN_NAME"

#
# Set base url in local etc/hosts en magento database
#
set_local_host() {
    if [ "$#" -gt 0 ]; then
        DOMAIN=$1
        save_properties
        shift
    fi

    if [[ "$#" -gt 0 && $1 != "--no-database" ]]; then
        modify_database=false
        shift
    fi

    #
    # Only write to /etc/hosts when something has to be written.
    #
    # A wildcard resolver for the TLD — ours, or one the machine already had — makes the entry
    # pointless, and the entry is what costs a password prompt per project and leaves a line
    # behind forever.
    #
    if hm_domain_resolves_locally "$DOMAIN"; then
        print_info "$DOMAIN already resolves to this machine, so /etc/hosts was left alone.\n"
    elif ! grep -qE "[[:space:]]$DOMAIN([[:space:]]|$)" /etc/hosts; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"
        echo "0.0.0.0 ::1 $DOMAIN $HM_HOSTS_MARKER" | sudo tee -a /etc/hosts
    fi

    if [[ -n "$DOMAIN" ]] && $modify_database; then
        print_info "Set "
        print_link "https://$DOMAIN/"
        print_info " to web/secure/base_url and web/secure/base_url.\n"

        # Add domain in core_config_data table
        "$COMMANDS_DIR"/magento.sh config:set web/secure/base_url https://"$DOMAIN"/
        "$COMMANDS_DIR"/magento.sh config:set web/unsecure/base_url https://"$DOMAIN"/
    fi
}

#
# Removes only what this tool added, and only for this domain. A line somebody wrote by hand has
# no marker, so it is not ours to delete.
#
remove_local_host() {
    local domain="${1:-$DOMAIN}"

    if [ -z "$domain" ]; then
        hm_fail "$HM_EXIT_USAGE" "no_domain" \
            "There is no domain to remove" \
            "$COMMAND_BIN_NAME set-host --remove <domain>"
    fi

    if ! grep -qE "[[:space:]]$domain([[:space:]]).*$HM_HOSTS_MARKER" /etc/hosts; then
        print_info "There is no entry for $domain that $COMMAND_BIN_NAME added.\n"
        return 0
    fi

    print_info "Your system password is needed to remove the entry from /etc/hosts...\n"

    local temporary
    temporary=$(mktemp) || return 1

    grep -vE "[[:space:]]$domain([[:space:]]).*$HM_HOSTS_MARKER" /etc/hosts > "$temporary"

    # Copied into place rather than moved: /etc/hosts has an owner, a mode and, on macOS, flags
    # that a rename from a temporary directory would not carry
    sudo cp "$temporary" /etc/hosts
    rm -f "$temporary"

    print_info "Removed the entry for $domain.\n"
}

arguments=()

for argument in "$@"; do
    case "$argument" in
        --remove) remove_entry=true ;;
        *)        arguments[${#arguments[@]}]="$argument" ;;
    esac
done

set -- ${arguments[@]+"${arguments[@]}"}

if $remove_entry; then
    remove_local_host "${1:-}"
    exit 0
fi

set_local_host "$@"