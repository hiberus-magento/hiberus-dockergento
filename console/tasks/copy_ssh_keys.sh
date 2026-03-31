#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "phpfpm"

if [[ -f ~/.ssh/id_rsa && -f ~/.ssh/id_rsa.pub ]]; then

  # Copy SSH Keys
  docker cp ~/.ssh "$($DOCKER_COMPOSE ps -q phpfpm | awk '{print $1}')":/var/www/.ssh

  # Apply permission fixes
  $DOCKER_COMPOSE exec -uroot phpfpm bash -c "chown app: /var/www/.ssh"
  $DOCKER_COMPOSE exec -uroot phpfpm bash -c "chmod -R 600 /var/www/.ssh"

  print_processing "SSH keys copied!"

fi

