#!/usr/bin/env bash

#
# What `hm setup` was told to do.
#
# Parsing is separated from doing for two reasons. The first is the bug it fixes: a dump path
# that does not exist used to be a warning, after which the command carried on and asked the
# question interactively — so an automated bootstrap with a wrong path hung instead of failing.
# Refusing it before anything is created is only possible if the reading happens first.
#
# The second is that this can then be tested. Creating an environment takes minutes and a working
# Docker; deciding what `--db-dump=./x.sql --clean-install` means takes neither, and that is where
# the bugs were.
#

#
# Assigns SETUP_DUMP, SETUP_INSTALL, SETUP_PROJECT_NAME, SETUP_DOMAIN, SETUP_ROOT,
# SETUP_FORCE, SETUP_USE_DEFAULT and SETUP_MAIL.
#
hm_setup_parse_options() {
    SETUP_DUMP=""
    SETUP_INSTALL=false
    SETUP_PROJECT_NAME=""
    SETUP_DOMAIN=""
    SETUP_ROOT=""
    SETUP_FORCE=false
    SETUP_USE_DEFAULT=false
    SETUP_MAIL=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            # `--clean-install` and `--db-dump` are Warden's names for the same two things. Half
            # the department has used it, and refusing the word somebody typed in order to be
            # tidy is being tidy at their expense
            -i | --install | --clean-install) SETUP_INSTALL=true; shift ;;

            -D | --dump | --db-dump) SETUP_DUMP="${2:-}"; shift 2 ;;
            --dump=* )               SETUP_DUMP="${1#--dump=}"; shift ;;
            --db-dump=* )            SETUP_DUMP="${1#--db-dump=}"; shift ;;

            -p | --project-name) SETUP_PROJECT_NAME="${2:-}"; shift 2 ;;
            --project-name=*)    SETUP_PROJECT_NAME="${1#--project-name=}"; shift ;;

            -d | --domain) SETUP_DOMAIN="${2:-}"; shift 2 ;;
            --domain=*)    SETUP_DOMAIN="${1#--domain=}"; shift ;;

            -r | --root-directory) SETUP_ROOT="${2:-}"; shift 2 ;;
            --root-directory=*)    SETUP_ROOT="${1#--root-directory=}"; shift ;;

            --mail=*) SETUP_MAIL="${1#--mail=}"; shift ;;
            --mail)   SETUP_MAIL="${2:-}"; shift 2 ;;

            -f | --force)       SETUP_FORCE=true; shift ;;
            -u | --use-default) SETUP_USE_DEFAULT=true; shift ;;

            -*)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                    "Unknown option: $1" \
                    "$COMMAND_BIN_NAME setup --domain=shop.test --clean-install"
                ;;
            *)
                hm_fail "$HM_EXIT_USAGE" "unexpected_argument" \
                    "'$1' is not something setup takes" \
                    "$COMMAND_BIN_NAME setup --help"
                ;;
        esac
    done

    #
    # A dump that is not there stops the command now, rather than warning and walking into a
    # question nobody is there to answer
    #
    if [ -n "$SETUP_DUMP" ]; then
        case "$SETUP_DUMP" in
            "~/"*) SETUP_DUMP="${HOME}/${SETUP_DUMP#\~/}" ;;
        esac

        if [ ! -f "$SETUP_DUMP" ]; then
            hm_fail "$HM_EXIT_USAGE" "dump_not_found" \
                "No such database dump: $SETUP_DUMP" \
                "$COMMAND_BIN_NAME setup --db-dump=/path/to/dump.sql"
        fi
    fi

    return 0
}
