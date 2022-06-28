#!/bin/bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh

sshHost="ssh.eu-3.magento.cloud"
sshPath="/app/pub/media"
sshUser=""

print_info "Media transfer assistant: \n"

for i in "$@"; do
    case $i in
    --ssh-host=*)
        sshHost="${i#*=}" && shift
        ;;
    --ssh-user=*)
        sshUser="${i#*=}" && shift
        ;;
    --ssh-path=*)
        sshPath="${i#*=}" && shift
        ;;
    -* | --* | *) ;;
    esac
done

# Request data
read -p "SSH Host [Default: '$sshHost']: " inputSshHost
read -p "SSH User [Default: '$sshUser']: " inputSshUser
read -p "SSH pub/media path [Default: '$sshPath']: " inputSshPath
sshHost=${inputSshHost:-${sshHost}}
sshUser=${inputSshUser:-${sshUser}}
sshPath=${inputSshPath:-${sshPath}}

if [ -z "$sshUser" ] || [ -z "$sshPath" ] || [ -z "$sshHost" ]; then
    print_error "Error: Please enter all required data\n"
    exit 1
fi

# Request confirmation
print_info "You are going to transfer files from [${sshHost}] to [LOCALHOST]. \nPress any key continue..."
read -r

# Check rsync data
if ! command -v rsync &>/dev/null; then
    print_error "Error: Rsync command not available\n"
    exit 1
fi

# Check local pub/media directory
if [ ! -d "./pub/media" ]; then
    print_error "Error: Local pub/media directory not found. Are you in the correct path?\n"
    exit 1
fi

# Transfer pub/media files
rsync -az --ignore-existing --exclude '**/cache' --max-size=10m ${sshUser}@${sshHost}:${sshPath} ./pub

print_info "All media content transferred! \n"
