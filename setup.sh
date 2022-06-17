#!/usr/bin/env bash

DIR="${HOME}/hiberus-dockergento";
EXECUTABLE="source ${DIR}/console/hm-completion.bash"
SOURCE_FILE="${HOME}/.zshrc"

# Compose string with all commands
COMMANDS=""
for script in "${DIR}/console/commands/"*.sh; do
    COMMAND_BASENAME=$(basename ${script})
    COMMAND_NAME=${COMMAND_BASENAME%.sh}
    COMMAND_DESC_PROPERTY="command_desc_${COMMAND_NAME//-/_}"
    COMMAND_DESC="${!COMMAND_DESC_PROPERTY:-}"
    COMMAND_OUTPUT=$(printf "  ${GREEN}%-30s${COLOR_RESET} %s" "${COMMAND_NAME}" "${COMMAND_DESC}")
    COMMANDS="${COMMANDS}${COMMAND_NAME} \\ \n"
done

# Write autocomplete file
echo -e "#/usr/bin/env bash\n\ncomplete -W \"${COMMANDS}\" hm" > "${DIR}"/console/hm-completion.bash

# Write source sentence in .zshrc
if ! grep -q "${EXECUTABLE}" $SOURCE_FILE; then
  echo "${EXECUTABLE}" >> $HOME/.zshrc
fi