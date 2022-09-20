#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

if [ "$#" -eq "0" ]; then
    DOMAIN="localhost"
else
    DOMAIN=$1
fi

if [ -z "$(docker ps | grep hitch)" ]; then
    print_error "Error: Hitch is not running!\n"
    exit
fi

print_info "$(printf "Generating SSL certificates for domain '%s'..." "${DOMAIN}")\n"

# Check if command "mkcert" exists
if ! command -v mkcert &>/dev/null; then
    print_error "Required 'mkcert' command not found. Trying to install...\n"

    # Install on MacOS
    if [ "$(uname)" == "Darwin" ]; then
        if ! command -v brew &>/dev/null; then
            print_error "Error: Brew is required. Please install it and try again.\n"
            exit 1
        fi
        brew install mkcert nss
        mkcert -install

    # Install on Linux
    else
        sudo apt-get -y install curl libnss3-tools
        curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
        chmod +x mkcert-v*-linux-amd64
        sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert
        mkcert -install

    fi
fi

if ! command -v mkcert &>/dev/null; then
    print_error "Error during 'mkcert' installation. Please do it manually and try again...\n"
    exit 1
fi

# Generate mkcert certificate
print_info "Installing SSL certificate into docker environment...\n"
mkcert -cert-file ssl.crt -key-file ssl.key ${DOMAIN} localhost 127.0.0.1 0.0.0.0 ::1
cat ssl.crt ssl.key >ssl.pem && rm ssl.crt ssl.key
docker cp ./ssl.pem "$(docker-compose ps -q hitch | awk '{print $1}')":/etc/hitch/testcert.pem
docker-compose exec -T -u root hitch chown hitch /etc/hitch/testcert.pem
docker-compose restart hitch

print_info "SSL certificate installed. Remember to restart your browser\n"
