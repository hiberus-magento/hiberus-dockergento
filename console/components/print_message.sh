#!/usr/bin/env bash

print_info() {
    printf "${GREEN}%b${COLOR_RESET}" "$1"
}

print_warning() {
    printf "${YELLOW}%b${COLOR_RESET}" "$1"
}

print_error() {
    printf "${RED}%b${COLOR_RESET}" "$1"
}

print_extra_data() {
    printf "${PURPLE}%b${COLOR_RESET}" "$1"
}

print_question() {
    printf "${BLUE}%b${COLOR_RESET}" "$1"
}

print_table() {
    printf "${CYAN}%b${COLOR_RESET}" "$1"
}

print_code() {
    printf "${BROWN}%b${COLOR_RESET}" "$1"
}

print_highlight() {
    printf "${WHITE}%b${COLOR_RESET}" "$1"
}

print_default() {
    printf "%b" "$1"
}