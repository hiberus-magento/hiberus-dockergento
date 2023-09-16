#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh
is_run_service "hitch"
domain="${DOMAIN:="localhost"}"

#
# Check if command "mkcert" exists
#
install_mkcert() {
    if ! command -v mkcert &>/dev/null; then
        print_error "Required 'mkcert' command not found. Trying to install...\n"

        # Install on MacOS
        if [ "$(uname)" == "Darwin" ]; then
            if ! command -v brew &>/dev/null; then
                print_error "Error: Brew is required. Please install it and try again.\n"
                exit 1
            fi
            brew install mkcert nss
        # Install on Linux
        else
            sudo apt-get -y install curl libnss3-tools
            curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
            chmod +x mkcert-v*-linux-amd64
            sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert
        fi
        mkcert -install
    fi

    if ! command -v mkcert &>/dev/null; then
        print_error "Error during 'mkcert' installation. Please do it manually and try again...\n"
        exit 1
    fi
}

#
# Prepare mkcert confifurations into container
#
setup_mkcert() {
    print_info "Installing SSL certificate into docker environment...\n"
    mkcert -cert-file ssl.crt -key-file ssl.key $domain localhost 127.0.0.1 0.0.0.0 ::1
    cat ssl.crt ssl.key >ssl.pem && rm ssl.crt ssl.key
    # Refactor to use in alternativo docker-compose path
    set +e
    docker cp "$MAGENTO_DIR"/ssl.pem "$(docker-compose ps -q hitch | awk '{print $1}')":/etc/hitch/testcert.pem
    "$COMMANDS_DIR"/exec.sh -T -u root hitch chown hitch /etc/hitch/testcert.pem

    "$COMMANDS_DIR"/restart.sh "hitch"
    set -e
    print_info "SSL certificate installed. Remember to restart your browser\n"
}

#
# Get domain
#
get_domain() {
    domain=${1:-$domain}

    print_info "Generating SSL certificates for domain "
    print_default "$domain"
    print_info "...\n"
}

get_domain "$@"
install_mkcert
setup_mkcert