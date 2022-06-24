#!/usr/bin/env bash

blue="\033[0;34m"
green="\033[0;32m"
red="\033[0;31m"
brown="\033[0;33m"
colorReset="\033[0m"

if ! command -v jq &>/dev/null; then
    echo -e "${green}Hiberus docker needs to install ${brown}jq${green} library${colorReset}"

    state="continue"
    while [[ "$state" == "continue" ]]; do
        printf "${blue}Do you want to install ${brown}jq${blue} in your computer? [y/n] ${colorReset}"
        read -r yn

        case $yn in
        [Nn]*)
            echo -e "${green}You can get information about jq instalation in:${colorReset}"
            echo -e "${blue}https://stedolan.github.io/jq/download/${colorReset}"
            exit 1 ;;
        [Yy]*)
            # Install on MacOS
            if [ "$(uname)" == "Darwin" ]; then
                if ! command -v brew &>/dev/null; then
                    echo -e "${red}Error: Brew is required. Please install it and try again.${colorReset}"
                    exit 1
                fi
                brew install jq
            # Install on Linux
            else
                sudo sudo apt-get install jq
            fi
            break
            ;;
        *) echo "Please answer yes or no." ;;
        esac
    done
fi

if [ -f ~/hm ]; then
    echo -e "${red}Hiberus docker already exits in your computer${colorReset}"
    exit 1
fi

git clone https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm
sudo ln -s "$HOME"/hm/bin/run /usr/local/bin/hm

# Execute generatt completion for autocomplete commands
"$HOME"/hm/generate_completion.sh