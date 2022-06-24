#!/usr/bin/env bash

print_info() {
    echo -e "${GREEN}$1${COLOR_RESET}"
}

print_warnning() {
    echo -e "${YELLOW}$1${COLOR_RESET}"
}

print_error() {
    echo -e "${RED}$1${COLOR_RESET}"
}

print_extra_data() {
    echo -e "${PURPLE}$1${COLOR_RESET}"
}

print_question() {
    echo -e "${BLUE}$1${COLOR_RESET}"
}

print_table() {
    echo -e "${CYAN}$1${COLOR_RESET}"
}

print_code() {
    echo -e "${BRONW}$1${COLOR_RESET}"
}

print_highlight() {
    echo -e "${WHITE}$1${COLOR_RESET}"
}