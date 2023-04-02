#!/usr/bin/env bash
set -euo pipefail

blue="\033[0;34m"
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
        echo "jq is not installed."

        # Check the operating system
        if [[ "$OSTYPE" == "darwin"* ]]; then
            echo "You are using Mac OS X."

            # Check if Homebrew is installed
            if ! [ -x "$(command -v brew)" ]; then
                echo "Homebrew is not installed."
                read -p "Do you want to install Homebrew? (y/n) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    # Install Homebrew
                    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                else
                    echo "Cannot install jq without Homebrew. Exiting script."
                    exit 1
                fi
            fi

            # Install jq using Homebrew
            brew install jq
        else
            echo "You are using Linux."

            # Install jq on Linux
            sudo apt-get update
            sudo apt-get install jq -y
        fi
    else
        echo "jq is already installed."
    fi
}

#
# Clone project if not extis
#
clone_project() {
    if [ ! -e ~/hm ]; then
        echo "clone_project" 
        git clone --branch 206-improve-instalation-in-mac-native-shell https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm
    fi
}

#
# Create binary link to hm command
#
create_link_to_command() {
   # Create /usr/local/bin if not exits
    if [ ! -d /usr/local/bin ]; then
        echo "exits local/bin"
        sudo mkdir -p /usr/local/bin
    fi

    # Link hm command
    if [ ! -e /usr/local/bin/hm ]; then
        echo "exits local/bin/hm"
        sudo ln -s "$HOME"/hm/bin/run /usr/local/bin/hm
    fi 
}

check_dependencies
clone_project
create_link_to_command
# Execute generate_completion.sh for autocomplete commands
"$HOME"/hm/generate_completion.sh