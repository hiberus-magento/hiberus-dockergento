#!/usr/bin/env bash

DIR=$(dirname -- "$(readlink -f -- "$0")")
EXECUTABLE="source ${DIR}/console/hm-completion.bash"
if [ "$(uname)" == "Darwin" ]; then
  SOURCE_FILE="${HOME}/.zshrc"
else
  SOURCE_FILE="${HOME}/.bashrc"
fi

# Show copy
source "${DIR}/console/tasks/copyright.sh"

# Compose string with all commands
COMMANDS=""
for script in "${DIR}/console/commands/"*.sh; do
  COMMAND_BASENAME=$(basename "${script}")
  COMMAND_NAME=${COMMAND_BASENAME%.sh}
  COMMANDS="${COMMANDS}${COMMAND_NAME} \\ \n"
done

# Write autocomplete file
echo -e "#!/usr/bin/env bash\n\ncomplete -W \"${COMMANDS}\" hm" >"${DIR}"/console/hm-completion.bash

# Write source sentence in .zshrc
if ! grep -q "${EXECUTABLE}" "$SOURCE_FILE"; then
  echo "${EXECUTABLE}" >>"$SOURCE_FILE"
fi
