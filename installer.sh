#!/usr/bin/env bash
set -euo pipefail

blue="\033[0;34m"
orange="\033[0;33m"
green="\033[0;32m"
red="\033[0;31m"
brown="\033[0;33m"
colorReset="\033[0m"

#
# Check if dependencies are installed before starting
#
check_dependencies() {
    # Check if jq is installed
    if ! [ -x "$(command -v jq)" ]; then
        echo -e "${green}jq is not installed.${colorReset}"
        read -p "$(echo -e "${blue}Do you want to install jq? (y/n) ${colorReset}")" -n 1 -r
        echo

        if ! [[ $REPLY =~ ^[Yy]$ ]]; then
            echo -e "${orange}\nCannot install Hiberus Magento CLI without jq !!!!\n${colorReset}"
            echo -e "${green}The installation process could not be completed.\n${colorReset}"
            exit 1  
        fi

        # Check the operating system
        if [[ "$OSTYPE" == "darwin"* ]]; then

            # Check if Homebrew is installed
            if ! [ -x "$(command -v brew)" ]; then
                echo -e "${green}Homebrew is not installed.${colorReset}"
                read -p "$(echo -e "${blue}Do you want to install Homebrew? (y/n) ${colorReset}")" -n 1 -r
                echo

                if ! [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo -e "${orange}Cannot install jq without Homebrew !!!!${colorReset}"
                    echo -e "${green}The installation process could not be completed.${colorReset}"
                    exit 1
                fi
                
                # Install Homebrew
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            fi

            # Install jq using Homebrew
            brew install jq
        else
            # Install jq on Linux
            sudo apt-get update
            sudo apt-get install jq -y
        fi
    else
       # Get the jq version and store it in a variable
        jq_version=$(jq --version | cut -d'-' -f2)

        # Split the version string into parts using a period as the separator
        IFS='.' read -ra version_parts <<< "$jq_version"

        # Check if the version is equal to or greater than 1.6
        if ! [[ ${version_parts[0]} -ge 1 && ${version_parts[1]} -ge 6 ]]; then
            echo -e "${orange}The jq version is less than 1.6${colorReset}"
        fi
    fi
}

#
# Clone project if not extis
#
clone_project() {
    if [ ! -e ~/hm ]; then
        git clone https://github.com/hiberus-magento/hiberus-dockergento.git "$HOME"/hm
    fi
    mkdir -p "$HOME/.hm_resources"
}

#
# Create binary link to hm command
#
create_link_to_command() {
   # Create /usr/local/bin if not exits
    if [ ! -d /usr/local/bin ]; then
        sudo mkdir -p /usr/local/bin
    fi

    # Link hm command
    if [ ! -e /usr/local/bin/hm ]; then
        sudo ln -s "$HOME"/hm/bin/run /usr/local/bin/hm
    fi 
}

check_dependencies
clone_project
create_link_to_command
# Execute generate_completion.sh for autocomplete commands
"$HOME"/hm/generate_completion.sh