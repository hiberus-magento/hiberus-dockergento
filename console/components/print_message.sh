#!/usr/bin/env bash

# Underlined blue for links, empty when colour is off: it follows the same decision as the
# rest of the palette instead of painting itself
if [ -n "${COLOR_RESET:-}" ]; then
    COLOR_LINK="\033[34;4m"
else
    COLOR_LINK=""
fi

# Colours normally come from load_properties.sh, but the components are also sourced
# standalone (tests, subshells). Default them to empty so `set -u` does not blow up and
# the output degrades to plain text.
BLUE="${BLUE:-}"
GREEN="${GREEN:-}"
CYAN="${CYAN:-}"
RED="${RED:-}"
PURPLE="${PURPLE:-}"
BROWN="${BROWN:-}"
WHITE="${WHITE:-}"
YELLOW="${YELLOW:-}"
COLOR_RESET="${COLOR_RESET:-}"

#
# In JSON mode stdout carries the response envelope, so decorative output (progress,
# warnings, informational messages) is routed to stderr and stripped of colour.
#
# print_question, print_default, print_table and print_code are excluded on purpose:
# they are used inside command substitutions to compose prompts and menus, and moving
# them off stdout would return empty strings.
#
_print_decorated() {
    local color="$1"
    local text="$2"

    if [[ "${HM_OUTPUT_FORMAT:-text}" == "json" ]]; then
        printf "%b" "$text" >&2
    else
        printf "$color%b$COLOR_RESET" "$text"
    fi
}

print_question() {
    local question=$1
    local default_value

    printf "$BLUE%b$COLOR_RESET" "$question"

    if [ $# -gt 1 ] && ([ "$2" != null ] && [ -n "$2" ]); then
        default_value=$2
        printf "$BLUE["
        print_default $default_value
        printf "$BLUE] $COLOR_RESET"
    fi
}

print_info() {
    _print_decorated "$GREEN" "$1"
}

print_warning() {
    _print_decorated "$YELLOW" "$1"
}

print_error() {
    _print_decorated "$RED" "$1"
}

print_extra_data() {
    _print_decorated "$PURPLE" "$1"
}

print_table() {
    printf "$CYAN%b$COLOR_RESET" "$1"
}

print_code() {
    printf "$BROWN%b$COLOR_RESET" "$1"
}

print_highlight() {
    _print_decorated "$WHITE" "$1"
}

print_default() {
    printf "$COLOR_RESET%b" "$1"
}

print_link() {
    printf "$COLOR_LINK%b$COLOR_RESET" "$1"
}

print_processing() {
    _print_decorated "$COLOR_RESET" "🚀 $1\n"
}

print_header() {
    _print_decorated "$WHITE" "========================================\n$1\n========================================\n"
}

# Versions with automatic newline (write to stderr to avoid contaminating command substitution)
print_info_line() {
    printf "$GREEN%b$COLOR_RESET\n" "$1" >&2
}

print_warning_line() {
    printf "$YELLOW%b$COLOR_RESET\n" "$1" >&2
}

print_error_line() {
    printf "$RED%b$COLOR_RESET\n" "$1" >&2
}

print_default_line() {
    printf "$COLOR_RESET%b\n" "$1" >&2
}