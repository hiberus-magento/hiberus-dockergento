#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$(docker ps | grep phpfpm)" ]; then
    print_error "Copy SSH Keys: Error: PHP is not running!\n"
    exit
fi

if [[ -f ~/.ssh/id_rsa && -f ~/.ssh/id_rsa.pub ]]; then

  # Copy SSH Keys
  docker cp ~/.ssh "$(docker-compose ps -q phpfpm | awk '{print $1}')":/var/www/.ssh

  # Apply permission fixes
  docker-compose exec -uroot phpfpm bash -c "chown app: /var/www/.ssh"
  docker-compose exec -uroot phpfpm bash -c "chmod -R 600 /var/www/.ssh"

  print_processing "SSH keys copied!"

fi

