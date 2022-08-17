#!/usr/bin/env bash

dir=$(dirname -- "$(readlink -f -- "$0")")
executable="source $dir/console/hm-completion.bash"

if [ "$(uname)" == "Darwin" ]; then
    if [ ! -z "$HOME/.zshrc" ]; then
        sourceFile="$HOME/.zshrc"
    else
        sourceFile="$HOME/.bash_profile"
    fi
else
    sourceFile="$HOME/.bashrc"
fi

# Show copy
source "$dir/console/tasks/copyright.sh"

# Compose string with all commands
commands=""
for script in "$dir/console/commands/"*.sh; do
    commandBaseName=$(basename "$script")
    commandName=${commandBaseName%.sh}
    commands="${commands}${commandName} \\ \n"
done

# Write autocomplete file
echo -e "#!/usr/bin/env bash\n\ncomplete -W \"$commands\" hm" >"$dir"/console/hm-completion.bash

# Write source sentence in .zshrc
if ! grep -q "$executable" "$sourceFile"; then
    echo "$executable" >>"$sourceFile"
fi
