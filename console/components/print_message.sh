#!/usr/bin/env bash

COLOR_LINK="\033[34;4m"

print_question() {
    local question=$1
    local default_value

    printf "$BLUE%b$COLOR_RESET" "$question"

    if [ $# -gt 1 ] && [ $2 != null ]; then
        default_value=$2
        printf "$BLUE["
        print_default $default_value
        printf "$BLUE] $COLOR_RESET"
    fi
}

print_info() {
    printf "$GREEN%b$COLOR_RESET" "$1"
}

print_warning() {
    printf "$YELLOW%b$COLOR_RESET" "$1"
}

print_error() {
    printf "$RED%b$COLOR_RESET" "$1"
}

print_extra_data() {
    printf "$PURPLE%b$COLOR_RESET" "$1"
}

print_table() {
    printf "$CYAN%b$COLOR_RESET" "$1"
}

print_code() {
    printf "$BROWN%b$COLOR_RESET" "$1"
}

print_highlight() {
    printf "$WHITE%b$COLOR_RESET" "$1"
}

print_default() {
    printf "$COLOR_RESET%b" "$1"
}

print_link() {
    printf "$COLOR_LINK%b$COLOR_RESET" "$1"
}

print_procesing() {
    print_default "🚀 $1\n"
}