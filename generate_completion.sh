#!/usr/bin/env bash
set -euo pipefail

source="${BASH_SOURCE[0]}"

while [ -h "$source" ]; do
    DIR=$(cd -P "$(dirname "$source")" && pwd)
    source="$(readlink "$source")"
    [[ $source != /* ]] && source="$DIR/$source"
done

dir="$(cd -P "$(dirname "$source")" && pwd)"
completion_dir="$HOME/.hm_resources)"

executable="source $completion_dir/console/hm-completion.bash"
exclude_commands="copy-from-container copy-to-container"

[[ "$(uname -s)" == "Darwin" ]] && mac_machine=true || mac_machine=false

if $mac_machine ; then
    if [ -f "$HOME/.zshrc" ]; then
        sourceFile="$HOME/.zshrc"
    else
        sourceFile="$HOME/.bash_profile"
    fi
else
    sourceFile="$HOME/.bashrc"
fi

# Show copy
source "$dir/console/tasks/copyright.sh"
source "$dir/console/helpers/array_manager.sh"

# Compose string with all commands
commands=""
for script in "$dir/console/commands/"*.sh; do
    command_base_name=$(basename "$script")
    command_name=${command_base_name%.sh}

    if $mac_machine || ! in_array "$command_name" "$exclude_commands"; then
        commands="${commands}${command_name} \\ \n"
    fi
done

if ! [ -d "$completion_dir" ]; then
    mkdir -p "$completion_dir"
fi

# Write autocomplete file
echo -e "#!/usr/bin/env bash\n\ncomplete -W \"$commands\" hm" > "$completion_dir"/hm-completion.bash

# Write source sentence in .zshrc
if ! grep -q "$executable" "$sourceFile"; then
    echo "$executable" >> "$sourceFile"
fi
