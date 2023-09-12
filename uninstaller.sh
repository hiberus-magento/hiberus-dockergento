#!/usr/bin/env bash
set -euo pipefail

blue="\033[0;34m"
green="\033[0;32m"
red="\033[0;31m"
brown="\033[0;33m"
colorReset="\033[0m"

#
# Remove completion line of main shell file
#
remove_completion() {
    local files=("$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.bashrc")

    [[ "$(uname -s)" == "Darwin" ]] && mac_machine=true || mac_machine=false
    
    for file in "${files[@]}"; do
        if $mac_machine ; then
            if [ -f "$file" ]; then
                sed -i '' '/source $dir/console/hm-completion.bash/d' "$file"
            fi
        else
            if [ -f "$file" ]; then
                sed -i '/source $dir/console/hm-completion.bash/d' "$file"
            fi
        fi
    done
   
    rm -f "$HOME"/console/hm-completion.bash
}

#
# Remove hm project
#
remove_project() {
    if ! [ -d "$HOME/hm" ]; then
        rm -r "$HOME/hm"
    fi
}

#
# Remove links to hm run project
#
remove_link_to_command() {
     if ! [ -d "/usr/local/bin/hm" ]; then
        rm -rf "/usr/local/bin/hm"
    fi
}

remove_completion
remove_link_to_command
remove_project
echo -e "$green Hiberus Magento CLI has been unistalled!!$colorReset"