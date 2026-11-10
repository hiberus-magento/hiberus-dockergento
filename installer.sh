#!/usr/bin/env bash
set -euo pipefail

blue="\033[0;34m"
orange="\033[0;33m"
green="\033[0;32m"
red="\033[0;31m"
brown="\033[0;33m"
colorReset="\033[0m"

# Where the hm command is linked; overridable so the link logic can be exercised without root
HM_LINK="${HM_LINK:-/usr/local/bin/hm}"

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
# Run a command as root only when it needs to be: the folder that holds the link may already be
# the user's own, as /usr/local/bin is under Homebrew
#
as_needed() {
    if [ -w "$(dirname "$HM_LINK")" ]; then
        "$@"
    else
        sudo "$@"
    fi
}

#
# Create binary link to hm command
#
create_link_to_command() {
    local home="$HOME/hm"
    local shell_entry="$home/bin/run"
    local binary_entry="$home/bin/hm"
    local link_dir
    link_dir="$(dirname "$HM_LINK")"

    # Create the folder that holds the link if it does not exist
    if [ ! -d "$link_dir" ]; then
        sudo mkdir -p "$link_dir"
    fi

    local target="$shell_entry"

    if [ -x "$binary_entry" ] || download_binary; then
        target="$binary_entry"
    else
        echo -e "${orange}Could not fetch the compiled command; using the shell one.${colorReset}"
    fi

    # Link hm command
    if [ ! -e "$HM_LINK" ] && [ ! -L "$HM_LINK" ]; then
        as_needed ln -s "$target" "$HM_LINK"
        return
    fi

    # A link this installer made earlier is moved to the current target: one made while hm was
    # shell only points at bin/run, and the shell no longer has the commands ported to the binary.
    # The reverse move happens too, when the binary could not be fetched this time
    local current
    current="$(readlink "$HM_LINK" || true)"

    if [ "$current" == "$target" ]; then
        return
    fi

    if [ "$current" != "$shell_entry" ] && [ "$current" != "$binary_entry" ]; then
        echo -e "${orange}$HM_LINK is not a link this installer made; left as it is. To use this install: sudo ln -sfn $target $HM_LINK${colorReset}"
        return
    fi

    if as_needed ln -sfn "$target" "$HM_LINK"; then
        echo -e "${green}$HM_LINK now points at $target${colorReset}"
    else
        echo -e "${red}Could not move $HM_LINK to $target; it still points at $current. Run: sudo ln -sfn $target $HM_LINK${colorReset}"
        return 1
    fi
}

check_dependencies
clone_project
create_link_to_command
# Execute generate_completion.sh for autocomplete commands
"$HOME"/hm/generate_completion.sh