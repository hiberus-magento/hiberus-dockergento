#!/bin/bash
set -euo pipefail

#
# Print al all commands info (native and custom)
#
print_commands_info() {
  local COMMANDS_OUTPUT=""
  local COMMANDS_PATH="${COMMANDS_DIR}"
  local FILE_CONTENT="${FILE}"
  local COMMAND_COLOR="${GREEN}"
  local TITLE="Command list"
  local UNDERLINE="------------\n"

  if [ $# -gt 0 ] && [ $1 == 'custom' ]; then
    COMMANDS_PATH="${CUSTOM_COMMANDS_DIR}"
    TITLE="Custom command list"
    UNDERLINE="-------------------\n"

    if [ -f "${CUSTOM_COMMANDS_DIR}/commands_description.json" ]; then
      FILE_CONTENT=å$(cat ${CUSTOM_COMMANDS_DIR}/commands_description.json)
    else
      FILE_CONTENT=$(echo "{}")
    fi
    
    COMMAND_COLOR="${PURPLE}"
  fi
  
  if [ ! -d "${COMMANDS_PATH}" ]; then
    exit 0
  fi

  local FILES=$(find "${COMMANDS_PATH}" -name '*.sh' | wc -l)
  
  if [ $FILES -gt 0 ]; then
    printf "${COMMAND_COLOR}\n${TITLE}${COLOR_RESET}\n"
    printf "${COMMAND_COLOR}${UNDERLINE}${COLOR_RESET}\n"
  
    for script in "${COMMANDS_PATH}"/*.sh; do
      COMMAND_BASENAME=$(basename ${script})
      COMMAND_NAME=${COMMAND_BASENAME%.sh}
      COMMAND_DESC_PROPERTY=$(echo "${FILE_CONTENT}" | jq -r 'if .'${COMMAND_NAME//-/_}'.description then .'${COMMAND_NAME//-/_}'.description else "" end')
      COMMAND_DESC="${!COMMAND_DESC_PROPERTY:-}"
      printf "   ${COMMAND_COLOR}%-20s${COLOR_RESET} %s\n" "${COMMAND_NAME}" "${COMMAND_DESC_PROPERTY}"
    done
    
    echo "\n"
  fi
}

#
# Print native commands and custom commands info
#
print_all_commands_help_info() {
  local COMMANDS_OUTPUT=$(print_commands_info)
  local COMMANDS_OUTPUT_ALL=$(print_commands_info "custom")
  printf "${COMMANDS_OUTPUT}"
  printf "${COMMANDS_OUTPUT_ALL}"
}

#
# Print arguments data array
#
print_opts() {
  local LENGTH=$(echo "${FILE}" | jq -r '.'$1'.opts | length')

  if [[ $LENGTH > 0 ]]; then
    printf "${YELLOW}Options:${COLOR_RESET}\n"
  fi

  for (( i=0; i<$LENGTH; i++ )); do
    name=$(echo "${FILE}" | jq -r '.'$1'.opts['$i'].name')
    description=$(echo "${FILE}" | jq -r '.'$1'.opts['$i'].description')
    printf "   ${GREEN}%-12s${COLOR_RESET}%s\n" "${name}" "${description}"
  done

  if [[ $LENGTH > 0 ]]; then
    printf "\n"
  fi
}

#
# Define usage
#
usage() {
  if [ $# == 0 ]; then
    print_all_commands_help_info
  else
    local COMMAND_NAME=$1
    COMMAND_NAME=${COMMAND_NAME//-/_}
    local PARAMS=$(echo "${FILE}" | jq -r '.'$COMMAND_NAME' | if length > 0 then keys[] else false end')

    if [[ $PARAMS ]]; then
      # Prinf usage seccion
      if [[ "$PARAMS" == *"usage"* ]]; then
        local usage=$(echo "${FILE}" | jq -r '.'$COMMAND_NAME'.usage')
        printf "${YELLOW}Usage:${COLOR_RESET}\n"
        printf "%3s${COMMAND_BIN_NAME} ${usage}\n\n"
      fi

      # Prinf example seccion
      if [[ "$PARAMS" == *"example"* ]]; then
        local example=$(echo "${FILE}" | jq -r '.'$COMMAND_NAME'.example')
        printf "${YELLOW}Example:${COLOR_RESET}\n"
        printf "%3s${COMMAND_BIN_NAME} ${example}\n\n"
      fi

      # Prinf description seccion
      if [[ "$PARAMS" == *"description"* ]]; then
        local description=$(echo "${FILE}" | jq -r '.'$COMMAND_NAME'.description')
        printf "${YELLOW}Description:${COLOR_RESET}\n"
        printf "%3s${description}\n\n"
      fi

      # Prinf options seccion
      if [[ "$PARAMS" == *"opts"* ]]; then
        print_opts $COMMAND_NAME
      fi
    fi
  fi
}

FILE="$(cat ${DATA_DIR}/command_descriptions.json)"

usage $@