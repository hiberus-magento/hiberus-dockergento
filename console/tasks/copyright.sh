#!/usr/bin/env bash

#
# Identity header.
#
# Three lines instead of eight: the header identifies the tool, it should not push what the
# user came for off the screen. Drawn with block characters when the locale says UTF-8, and
# with ASCII otherwise, because a logo rendered as question marks is worse than no logo.
#
# Decoration, so it is not printed when the output is not a terminal.
#

print_logo() {
    [ -t 1 ] || return 0

    local charset="${LC_ALL:-${LC_CTYPE:-${LANG:-}}}"

    printf "%b" "${BLUE:-}"

    case "$charset" in
        *UTF-8* | *utf-8* | *UTF8* | *utf8*)
            cat <<'LOGO'
█  █ █▀▄▀█   Hiberus Dockergento
█▀▀█ █ ▀ █   Docker environments for Magento 2
▀  ▀ ▀   ▀
LOGO
            ;;
        *)
            cat <<'LOGO'
H   H  M   M   Hiberus Dockergento
HHHHH  M M M   Docker environments for Magento 2
H   H  M   M
LOGO
            ;;
    esac

    printf "%b" "${COLOR_RESET:-}"
    printf '\n'
}
