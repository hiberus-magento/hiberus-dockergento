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
        git clone https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm
    fi
}

#
# Fetch the compiled entry point for this machine.
#
# From 2.0 the command is a binary that runs the shell implementation for everything not ported
# yet. The installation is still the git checkout — that is how `hm switch` and `hm update` work —
# so this adds exactly one file to it.
#
# A machine that cannot get the binary is not left without a tool: the link falls back to the
# shell implementation, which is the whole of 1.x and most of 2.0.
#
download_binary() {
    local system architecture asset

    case "$(uname -s)" in
        Darwin) system="darwin" ;;
        Linux)  system="linux" ;;
        *)      return 1 ;;
    esac

    case "$(uname -m)" in
        arm64 | aarch64) architecture="arm64" ;;
        x86_64 | amd64)  architecture="amd64" ;;
        *)               return 1 ;;
    esac

    asset="https://github.com/hiberus-magento/hiberus-dockergento/releases/latest/download/hm_${system}_${architecture}"

    curl -fsSL "$asset" -o "$HOME/hm/bin/hm.download" 2>/dev/null || return 1
    chmod +x "$HOME/hm/bin/hm.download"

    # Renamed once it is whole: an interrupted download must not look like an entry point
    mv "$HOME/hm/bin/hm.download" "$HOME/hm/bin/hm"
}

#
# Create binary link to hm command
#
create_link_to_command() {
   # Create /usr/local/bin if not exits
    if [ ! -d /usr/local/bin ]; then
        sudo mkdir -p /usr/local/bin
    fi

    local target="$HOME/hm/bin/run"

    if [ -x "$HOME/hm/bin/hm" ] || download_binary; then
        target="$HOME/hm/bin/hm"
    else
        echo -e "${orange}Could not fetch the compiled command; using the shell one.${colorReset}"
    fi

    # Link hm command
    if [ ! -e /usr/local/bin/hm ]; then
        sudo ln -s "$target" /usr/local/bin/hm
    fi
}

check_dependencies
clone_project
create_link_to_command
# Execute generate_completion.sh for autocomplete commands
"$HOME"/hm/generate_completion.sh