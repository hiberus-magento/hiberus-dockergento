#!/usr/bin/env bash

# Check if command "jq" exists
if ! command -v jq  &> /dev/null
then
    printf "${RED}Required 'jq' not found${COLOR_RESET}"
    printf "${BLUE}https://stedolan.github.io/jq/download/${COLOR_RESET}"
    exit
fi

echo $(cat "${DATA_DIR}/requeriments.json" | jq -r 'keys|join(" ")')