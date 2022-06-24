#!/bin/bash
set -euo pipefail

sshHost="ssh.eu-3.magento.cloud"
sshPath="/app/pub/media"
sshUser=""

printf "${GREEN}Media transfer assistant: ${COLOR_RESET}\n"

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
read -p "SSH Host [Default: '${sshHost}']: " inputSshHost
read -p "SSH User [Default: '${sshUser}']: " inputSshUser
read -p "SSH pub/media path [Default: '${sshPath}']: " inputSshPath
sshHost=${inputSshHost:-${sshHost}}
sshUser=${inputSshUser:-${sshUser}}
sshPath=${inputSshPath:-${sshPath}}

if [ -z "$sshUser" ] || [ -z "$sshPath" ] || [ -z "$sshHost" ]; then
    printf "${RED}Error: Please enter all required data${COLOR_RESET}\n"
    exit 1
fi

# Request confirmation
printf "${GREEN}You are going to transfer files from [${sshHost}] to [LOCALHOST]. ${COLOR_RESET}\nPress any key continue..."
read

# Check rsync data
if ! command -v rsync &>/dev/null; then
    printf "${RED}Error: Rsync command not available${COLOR_RESET}\n"
    exit 1
fi

# Check local pub/media directory
if [ ! -d "./pub/media" ]; then
    printf "${RED}Error: Local pub/media directory not found. Are you in the correct path?${COLOR_RESET}\n"
    exit 1
fi

# Transfer pub/media files
rsync -az --ignore-existing --exclude '**/cache' --max-size=10m ${sshUser}@${sshHost}:${sshPath} ./pub

printf "${GREEN}All media content transferred! ${COLOR_RESET}\n"
