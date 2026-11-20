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

# Where releases publish their assets: hm_<os>_<arch> and checksums.txt
RELEASES="https://github.com/hiberus-magento/hiberus-dockergento/releases"

#
# The compiled command for this machine.
#
# From 2.0 the command is the binary bin/hm. The installation is still the git checkout — that is
# how `hm switch` and `hm update` work — and bin/hm is not in git, so it is obtained here: the
# release asset of the tag the checkout is at, verified against that release's checksums.txt, or
# built with a local Go toolchain. A checkout that is not at a tag is only ever built, as hm update
# and hm switch do: the latest release's binary would be another version than the files beside it.
#
# There is no fallback to a shell entry point: bin/run, the launcher hm used to be, is gone.
#
release_asset() {
    local system architecture

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

    echo "hm_${system}_${architecture}"
}

sha256_of() {
    if [ -x "$(command -v sha256sum)" ]; then
        sha256sum "$1" | cut -d' ' -f1
    elif [ -x "$(command -v shasum)" ]; then
        shasum -a 256 "$1" | cut -d' ' -f1
    else
        return 1
    fi
}

# download_binary <tag>: the asset of that release, or of the latest one when the tag is empty.
# Fail closed: no checksums, no line for the asset, or a different digest is no binary.
download_binary() {
    local tag="$1" asset base expected actual
    local partial="$HOME/hm/bin/hm.download"
    local manifest="$HOME/hm/bin/checksums.download"

    asset="$(release_asset)" || return 1

    if [ -n "$tag" ]; then
        base="$RELEASES/download/$tag"
    else
        base="$RELEASES/latest/download"
    fi

    mkdir -p "$HOME/hm/bin"

    if ! curl -fsSL "$base/$asset" -o "$partial" 2>/dev/null ||
        ! curl -fsSL "$base/checksums.txt" -o "$manifest" 2>/dev/null; then
        rm -f "$partial" "$manifest"
        return 1
    fi

    expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1 }' "$manifest")"
    actual="$(sha256_of "$partial" || true)"
    rm -f "$manifest"

    if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
        echo -e "${orange}The downloaded $asset does not match the release's checksums; it was discarded.${colorReset}"
        rm -f "$partial"
        return 1
    fi

    chmod +x "$partial"

    # Renamed once it is whole and verified: an interrupted download must not look like an entry point
    mv "$partial" "$HOME/hm/bin/hm"
}

build_binary() {
    local partial="$HOME/hm/bin/hm.build"

    [ -x "$(command -v go)" ] || return 1

    echo -e "${green}Building the hm binary with $(command -v go)...${colorReset}"

    if ! (cd "$HOME/hm" && go build -o bin/hm.build ./cmd/hm); then
        rm -f "$partial"
        return 1
    fi

    chmod +x "$partial"
    mv "$partial" "$HOME/hm/bin/hm"
}

obtain_binary() {
    local tag
    tag="$(git -C "$HOME/hm" describe --tags --exact-match HEAD 2>/dev/null || true)"

    if [ -n "$tag" ]; then
        download_binary "$tag" || build_binary
    else
        build_binary
    fi
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

    # Before anything else is touched: without a binary there is nothing to link, and an existing
    # installation is left exactly as it is
    if [ ! -x "$binary_entry" ] && ! obtain_binary; then
        echo -e "${red}Could not obtain the hm binary for $(uname -s)/$(uname -m): no release binary could be downloaded and verified, and there is no Go toolchain to build it.${colorReset}" >&2
        echo -e "${red}Install Go (https://go.dev/dl/) and run this installer again, or build it yourself: cd $home && go build -o bin/hm ./cmd/hm${colorReset}" >&2
        if [ -z "$(git -C "$home" describe --tags --exact-match HEAD 2>/dev/null)" ]; then
            echo -e "${red}Without Go, only a release has a binary to download: pin one and run this installer again: git -C $home checkout <version>   (versions: git -C $home tag)${colorReset}" >&2
        fi
        if [ -e "$HM_LINK" ] || [ -L "$HM_LINK" ]; then
            echo -e "${orange}$HM_LINK was left as it was.${colorReset}" >&2
        fi
        exit 1
    fi

    local target="$binary_entry"

    # Create the folder that holds the link if it does not exist
    if [ ! -d "$link_dir" ]; then
        sudo mkdir -p "$link_dir"
    fi

    # Link hm command
    if [ ! -e "$HM_LINK" ] && [ ! -L "$HM_LINK" ]; then
        as_needed ln -s "$target" "$HM_LINK"
        return
    fi

    # A link this installer made earlier is moved to the binary: one made while hm was shell only
    # points at bin/run, which no longer exists
    local current
    current="$(readlink "$HM_LINK" || true)"

    if [ "$current" == "$target" ]; then
        return
    fi

    if [ "$current" != "$shell_entry" ]; then
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