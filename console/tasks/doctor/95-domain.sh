#!/usr/bin/env bash
# Does this project's domain reach this machine, and how?
source "$HELPERS_DIR"/doctor.sh
source "$HELPERS_DIR"/domain_resolution.sh
source "$HELPERS_DIR"/hosts.sh
doctor_requires_project

domain="${DOMAIN:-}"

if [ -z "$domain" ]; then
    doctor_warning "This project has no domain configured" "$COMMAND_BIN_NAME setup --domain=project.test"
    return 0 2>/dev/null || exit 0
fi

#
# A domain can resolve through /etc/hosts and still be unreachable, so this is asked before the
# source: the entry earlier versions wrote points at 0.0.0.0, an address Firefox refuses, and the
# hosts file is what answers whatever else could have resolved the name.
#
if hm_hosts_has_legacy /etc/hosts "$domain"; then
    doctor_warning "$domain resolves to 0.0.0.0 in /etc/hosts, which Firefox refuses to connect to" \
        "$COMMAND_BIN_NAME set-host $domain --no-database"
    return 0 2>/dev/null || exit 0
fi

case "$(hm_domain_resolution_source "$domain")" in
    dns)
        doctor_ok "$domain resolves here through DNS, with no /etc/hosts entry"
        ;;
    hosts)
        doctor_ok "$domain resolves here through /etc/hosts"
        ;;
    *)
        doctor_error "$domain does not resolve to this machine" \
            "$COMMAND_BIN_NAME set-host $domain"
        ;;
esac
