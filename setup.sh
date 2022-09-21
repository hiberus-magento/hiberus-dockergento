#!/usr/bin/env bash

blue="\033[0;34m"
green="\033[0;32m"
red="\033[0;31m"
brown="\033[0;33m"
colorReset="\033[0m"

if [ -f ~/hm ]; then
    echo -e "${red}Hiberus docker already exits in your computer${colorReset}"
    exit 1
fi

git clone https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm
sudo ln -s "$HOME"/hm/bin/run /usr/local/bin/hm

# Execute generatt completion for autocomplete commands
"$HOME"/hm/generate_completion.sh

if ! command -v jq &>/dev/null; then
   if [ "$(uname)" == "Darwin" ]; then
        # Install on Mac
        if ! command -v brew &>/dev/null; then
            echo -e "${red}Error: Brew is required. Please install it and try again.${colorReset}"
            exit 1
        fi
        brew install jq
    else
        # Install on Linux
        sudo sudo apt-get -y install jq
    fi
fi
