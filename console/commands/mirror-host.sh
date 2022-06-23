#!/usr/bin/env bash
set -euo pipefail

#
# Validate mirror host path
#
validate_mirror_host_path() {
    PATH_TO_MIRROR=$1

    BIND_MOUNT_PATH=$("${TASKS_DIR}/get_bind_mount_path.sh" "${WORKDIR_PHP}/${PATH_TO_MIRROR}")
    if [[ ${BIND_MOUNT_PATH} != false ]]; then
        echo ""
        echo -e "${RED}Path cannot be mirrored. Following path is a bind mount inside container:${COLOR_RESET}"
        echo ""
        echo "  - ./<host_path>:${BIND_MOUNT_PATH}"
        echo ""
        exit 1
    fi
}

if [[ "${MACHINE}" != "mac" ]]; then
    echo -e "${RED} This command is only for mac system.${COLOR_RESET}"
    exit 1
fi

source ${TASKS_DIR}/mirror_path.sh

echo -e "${GREEN}Start mirror copy of host into container${COLOR_RESET}"
CONTAINER_ID=$(${DOCKER_COMPOSE} ps -q phpfpm)

for PATH_TO_MIRROR in "$@"; do
    echo -e "${YELLOW}${PATH_TO_MIRROR} -> phpfpm:${PATH_TO_MIRROR}${COLOR_RESET}\n"

    echo " > validating and sanitizing path: '${PATH_TO_MIRROR}'"
    PATH_TO_MIRROR=$(sanitize_mirror_path "${PATH_TO_MIRROR}")
    validate_mirror_host_path "${PATH_TO_MIRROR}"

    SRC_PATH=${PATH_TO_MIRROR}
    DEST_PATH=${PATH_TO_MIRROR}
    DEST_DIR=$(dirname "${DEST_PATH}")

    SRC_IS_DIR=$([ -d "${SRC_PATH}" ] && echo true || echo false)
    if [[ ${SRC_IS_DIR} == *true* ]]; then
        echo " > removing destination dir content: 'phpfpm:${DEST_PATH}/*'"
        ${COMMAND_BIN_NAME} exec sh -c "rm -rf ${DEST_PATH}/*"
        SRC_PATH="${SRC_PATH}/."
        DEST_DIR="${DEST_PATH}"
    fi

    echo " > ensure destination dir exists: '${DEST_DIR}'"
    ${COMMAND_BIN_NAME} exec sh -c "mkdir -p ${DEST_DIR}"

    if [[ ${SRC_IS_DIR} == *true* && $(find "${SRC_PATH}" -maxdepth 0 -empty) ]]; then
        echo " > skipping copy. Source dir is empty: '${SRC_PATH}'"
    else
        echo " > copying '${SRC_PATH}' into 'phpfpm:${WORKDIR_PHP}/${DEST_PATH}'"
        docker cp ${SRC_PATH} ${CONTAINER_ID}:${WORKDIR_PHP}/${DEST_PATH}
    fi

    OWNERSHIP_COMMAND="chown -R ${USER_PHP}:${GROUP_PHP} ${WORKDIR_PHP}/${DEST_PATH}"
    echo " > setting permissions: ${OWNERSHIP_COMMAND}"
    ${COMMAND_BIN_NAME} exec --root sh -c "${OWNERSHIP_COMMAND}"
done

echo -e "${GREEN}Host mirrored into container${COLOR_RESET}"
