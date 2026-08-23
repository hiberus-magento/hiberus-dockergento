#!/usr/bin/env bash
set -euo pipefail

source="${BASH_SOURCE[0]}"

while [ -h "$source" ]; do
    DIR=$(cd -P "$(dirname "$source")" && pwd)
    source="$(readlink "$source")"
    [[ $source != /* ]] && source="$DIR/$source"
done

dir="$(cd -P "$(dirname "$source")" && pwd)"

executable="source $dir/console/hm-completion.bash"
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

# Write autocomplete file
echo -e "#!/usr/bin/env bash\n\ncomplete -W \"$commands\" hm" > "$dir"/console/hm-completion.bash

#
# Register it in the shell profile — but only for the installation that is actually on PATH.
#
# Any other checkout —a second clone, a test lab, a throwaway directory— regenerates its own
# completion file and stops there. Writing a line for it would point the user's shell at a
# directory that is about to disappear, and since the old check was "is this exact line already
# present", every new path added a new line: a profile that accumulated one dead `source` per
# clone, each one an error on every new terminal.
#
installed_target=$(command -v "$(basename "${COMMAND_BIN_NAME:-hm}")" 2>/dev/null || true)

if [ -n "$installed_target" ]; then
    while [ -h "$installed_target" ]; do
        link_dir=$(cd -P "$(dirname "$installed_target")" && pwd)
        installed_target="$(readlink "$installed_target")"
        [[ $installed_target != /* ]] && installed_target="$link_dir/$installed_target"
    done
    # bin/run -> the installation directory
    installed_dir=$(cd -P "$(dirname "$(dirname "$installed_target")")" 2>/dev/null && pwd) || installed_dir=""
else
    installed_dir=""
fi

if [ -n "$installed_dir" ] && [ "$installed_dir" != "$dir" ]; then
    exit 0
fi

[ -f "$sourceFile" ] || touch "$sourceFile"

# Drop any previous line —this path or another— before adding the current one, so moving or
# reinstalling the tool replaces its registration instead of stacking a new one on top
if grep -q "console/hm-completion.bash" "$sourceFile"; then
    profile_without_completion=$(grep -v "console/hm-completion.bash" "$sourceFile")
    printf '%s\n' "$profile_without_completion" > "$sourceFile"
fi

echo "$executable" >> "$sourceFile"
