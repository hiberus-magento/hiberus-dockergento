#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq  "0" ]; then
  DOMAIN="localhost"
else
  DOMAIN=$1
fi

CERT_FILE="${DOMAIN}.crt"
CERT_NAME="${DOMAIN}"

if [ -z "$(docker ps|grep hitch)" ]; then
  printf "${RED}Error: Hitch is not running!${COLOR_RESET}\n"
  exit
fi

printf "${GREEN}Generating SSL certificates for domain '${DOMAIN}'...${COLOR_RESET}\n"

# Check if command "mkcert" exists
if ! command -v mkcert  &> /dev/null
then
    printf "${RED}Required 'mkcert' command not found. Trying to install...${COLOR_RESET}\n"

    # Install on MacOS
    if [ "$(uname)" == "Darwin" ]; then
      sudo brew install mkcert nss
      mkcert -install

    # Install on Linux
    else
      if ! command -v curl  &> /dev/null; then
        sudo apt-get -y install curl
      fi
      curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
      chmod +x mkcert-v*-linux-amd64
      sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert
      mkcert -install

    fi
fi

if ! command -v mkcert  &> /dev/null; then
  printf "${RED}Error during 'mkcert' installation. Please do it manually and try again...${COLOR_RESET}\n"
  exit 1
fi

# Generate mkcert certificate
printf "${GREEN}Installing SSL certificate into docker environment...${COLOR_RESET}\n"
mkcert -cert-file ssl.crt -key-file ssl.key ${DOMAIN} localhost 127.0.0.1 0.0.0.0 ::1
cat ssl.crt ssl.key > ssl.pem && rm ssl.crt ssl.key
docker cp ./ssl.pem "$(docker-compose ps -q hitch|awk '{print $1}')":/etc/hitch/testcert.pem
docker-compose exec -T -u root hitch chown hitch /etc/hitch/testcert.pem
docker-compose restart hitch

printf "${GREEN}SSL certificate installed! Remember to restart your browser${COLOR_RESET}\n"
