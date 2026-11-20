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

# Where the tool is cloned from; overridable so choosing a version can be exercised offline
HM_REPOSITORY="${HM_REPOSITORY:-https://github.com/hiberus-magento/hiberus-dockergento.git}"

# The version to install, a tag or a branch, as the first argument (curl ... | bash -s -- 2.0.0) or
# in HM_VERSION. Empty is the latest release.
HM_VERSION="${1:-${HM_VERSION:-}}"

# is_go_version <checkout> <revision>: whether that revision's tree has the Go program. Read from
# the tree and not from a version number, because a branch has none: 2.0 is the first version with
# cmd/hm, and 1.x, the shell tool, has none.
is_go_version() {
    git -C "$1" cat-file -e "$2:cmd/hm/main.go" 2>/dev/null
}

# revision_of <checkout> <reference>: the tag or origin's branch of that name, or nothing.
revision_of() {
    if git -C "$1" rev-parse --verify --quiet "refs/tags/$2" >/dev/null; then
        echo "refs/tags/$2"
    elif git -C "$1" rev-parse --verify --quiet "refs/remotes/origin/$2" >/dev/null; then
        echo "refs/remotes/origin/$2"
    fi

    return 0
}

refuse_shell_version() {
    echo -e "${red}'$1' is a version of the shell tool (1.x); this installer installs 2.0 and later only. Nothing was installed.${colorReset}" >&2
    echo -e "${red}Install it separately, in a directory of its own: git clone -b $1 $HM_REPOSITORY ~/hm-1.x, then run ~/hm-1.x/bin/run${colorReset}" >&2
    exit 2
}

# latest_release <checkout>: the newest final version with the Go program, or, while there is none,
# the newest pre-release with it. Never a 1.x: those have no cmd/hm.
latest_release() {
    local pass tag

    for pass in final pre-release; do
        while IFS= read -r tag; do
            [[ "$tag" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]] || continue
            if [ "$pass" == "final" ] && [[ "$tag" == *-* ]]; then continue; fi
            if [ "$pass" == "pre-release" ] && [[ "$tag" != *-* ]]; then continue; fi

            if is_go_version "$1" "refs/tags/$tag"; then
                echo "$tag"
                return 0
            fi
        done < <(git -C "$1" tag --list --sort=-version:refname)
    done

    return 1
}

#
# The checkout at ~/hm, at the version asked for or at the latest release.
#
# A version of the shell tool is refused before anything is installed: the clone made for looking
# at it is removed again. An existing ~/hm is kept as it is, at whatever it has checked out: hm
# switch is what moves an installation.
#
clone_project() {
    local home="$HOME/hm" revision tag

    if [ -e "$home" ]; then
        if [ -n "$HM_VERSION" ]; then
            revision="$(revision_of "$home" "$HM_VERSION")"
            if [ -n "$revision" ] && ! is_go_version "$home" "$revision"; then
                refuse_shell_version "$HM_VERSION"
            fi
            echo -e "${orange}$home already exists and was left as it is; move it with: hm switch $HM_VERSION${colorReset}"
        fi
        return
    fi

    git clone --quiet --no-checkout "$HM_REPOSITORY" "$home"

    if [ -n "$HM_VERSION" ]; then
        revision="$(revision_of "$home" "$HM_VERSION")"

        if [ -z "$revision" ]; then
            rm -rf "$home"
            echo -e "${red}There is no version or branch called '$HM_VERSION'. Nothing was installed.${colorReset}" >&2
            exit 2
        fi

        if ! is_go_version "$home" "$revision"; then
            rm -rf "$home"
            refuse_shell_version "$HM_VERSION"
        fi

        case "$revision" in
            refs/tags/*) git -C "$home" checkout --quiet "$HM_VERSION" ;;
            *)           git -C "$home" checkout --quiet -B "$HM_VERSION" --track "origin/$HM_VERSION" ;;
        esac
        return
    fi

    if ! tag="$(latest_release "$home")"; then
        rm -rf "$home"
        echo -e "${red}$HM_REPOSITORY has no release of 2.0 or later. Nothing was installed.${colorReset}" >&2
        exit 1
    fi

    git -C "$home" checkout --quiet "$tag"
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

# The version first: one that is refused is refused before anything is installed
clone_project
check_dependencies
create_link_to_command
# Execute generate_completion.sh for autocomplete commands
"$HOME"/hm/generate_completion.sh