#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/properties.sh
source "$HELPERS_DIR"/domain_resolution.sh
source "$HELPERS_DIR"/hosts.sh
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
# Rewrites /etc/hosts with the entries this tool should have for the domain.
#
# Through a copy, never edited in place: `sed -i` is not portable — BSD sed takes the argument
# after `-i` as the backup extension, so `sed -i -E` on macOS loses the extended regex, deletes
# nothing, exits 0 and leaves /etc/hosts-E behind — and a rename from a temporary directory would
# not carry the owner, the mode and, on macOS, the file flags this one has.
#
write_hosts_entries() {
    local temporary
    temporary=$(mktemp) || return 1

    hm_hosts_repaired /etc/hosts "$DOMAIN" "$HM_HOSTS_MARKER" > "$temporary"

    # Never over an empty file: this one is how the machine resolves every name it knows
    if [ -s "$temporary" ]; then
        sudo cp "$temporary" /etc/hosts
    else
        print_error "/etc/hosts could not be read, so it was left alone.\n"
    fi

    rm -f "$temporary"
}

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
    # The malformed entry earlier versions wrote is asked about first, and before anything a
    # resolver says: the hosts file answers first, so a domain with that line in it resolves to
    # 0.0.0.0 whatever else could have resolved it. Why that address is a problem, and what
    # replaces it, is in helpers/hosts.sh.
    if hm_hosts_has_legacy /etc/hosts "$DOMAIN"; then
        print_info "Your system password is needed to repair the entry in /etc/hosts...\n"
        write_hosts_entries
    elif hm_domain_resolves_locally "$DOMAIN"; then
        print_info "$DOMAIN already resolves to this machine, so /etc/hosts was left alone.\n"
    elif ! hm_hosts_has_domain /etc/hosts "$DOMAIN"; then
        print_info "Your system password is needed to add an entry to /etc/hosts...\n"
        write_hosts_entries
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

    if ! grep -qE "$(hm_hosts_marked_pattern "$domain" "$HM_HOSTS_MARKER")" /etc/hosts; then
        print_info "There is no entry for $domain that $COMMAND_BIN_NAME added.\n"
        return 0
    fi

    print_info "Your system password is needed to remove the entry from /etc/hosts...\n"

    local temporary
    temporary=$(mktemp) || return 1

    grep -vE "$(hm_hosts_marked_pattern "$domain" "$HM_HOSTS_MARKER")" /etc/hosts > "$temporary"

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