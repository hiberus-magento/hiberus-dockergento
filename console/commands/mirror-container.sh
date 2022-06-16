#!/usr/bin/env bash
set -euo pipefail

[ "$#" -eq 0 ] && echo "Please specify a directory or file to copy from container (ex. vendor, --all)" && exit

CONTAINER_ID=$(docker-compose ps -q ${SERVICE_PHP})

if [ "$1" == "--all" ]; then
	REF_PATH="./"
	DEST_PATH="."
  FILES="all files"
else
	REF_PATH=$1
	FILES=$1
  if [ -f "$1" ] ; then
		DEST_PATH=$1
  else
		DEST_PATH=$(dirname "$1")
  fi
fi

docker cp ${CONTAINER_ID}:${WORKDIR_PHP}/"${REF_PATH}" "${DEST_PATH}"

printf "${GREEN}Completed copying ${FILES} from container to host\n${COLOR_RESET}"