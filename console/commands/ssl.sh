#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq  "0" ]; then
  DOMAIN="localhost"
else
  DOMAIN=$1
fi
CERT_FILE="${DOMAIN}.pem"
CERT_NAME="Hiberus Local Cert: ${DOMAIN}"

if [ -z "$(docker ps|grep hitch)" ]; then
  printf "${RED}Error: Hitch is not running!${COLOR_RESET}\n"
  exit
fi

printf "${GREEN}Generating SSL certificates for domain '${DOMAIN}'...${COLOR_RESET}\n"

# Generate SSL certificate in Hitch container
docker-compose exec -T -u root hitch openssl req -newkey rsa:2048 -sha256 -keyout testcert.key -nodes -x509 -days 365 -out testcert.crt -subj "/C=ES/ST=Spain/L=Spain/O=Hiberus/OU=Hiberus Magento/CN=${DOMAIN}" && cat testcert.key testcert.crt > /etc/hitch/testcert.pem && chown hitch /etc/hitch/testcert.pem
docker-compose restart hitch

# Get generated SSL certificate
docker cp "$(docker-compose ps -q hitch|awk '{print $1}')":/etc/hitch/testcert.pem ${CERT_FILE}

printf "${GREEN}Installing SSL certificate into local environment...${COLOR_RESET}\n"

# Install on MacOS
if [ "$(uname)" == "Darwin" ]; then
  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${CERT_FILE}
  FFoxBin="/Applications/Firefox.app/Contents/MacOS/firefox-bin"
  if [ -f "$FFoxBin" ]; then
    echo "{\"policies\": {\"Certificates\": {\"ImportEnterpriseRoots\": true}}}" | sudo tee policies.json
    DistDirectory="/Applications/Firefox.app/Contents/Resources/distribution"
    if [ ! -d "$DistDirectory" ]; then
      sudo mkdir -p "$DistDirectory"
    fi
    sudo mv policies.json "$DistDirectory"/policies.json
    CertDirectory="/Library/Application Support/Mozilla/Certificates"
    if [ ! -d "$CertDirectory" ]; then
      sudo mkdir -p "$CertDirectory"
    fi
    sudo mv ${CERT_FILE} "$CertDirectory"/${CERT_FILE}
  else
    sudo rm ${CERT_FILE}
  fi

# Install on Linux
else
  REQUIRED_PKG="libnss3-tools"
  PKG_OK=$(dpkg-query -W --showformat='${Status}\n' $REQUIRED_PKG|grep "install ok installed")
  echo Checking for $REQUIRED_PKG: "$PKG_OK"
  if [ "" = "$PKG_OK" ]; then
    echo "No $REQUIRED_PKG found. Setting up $REQUIRED_PKG."
    sudo apt-get --yes install $REQUIRED_PKG
  fi
  find ~/ -name "cert8.db" -print0 | while read -r certDB
  do
      certdir=$(dirname "${certDB}");
      certutil -D -n "${CERT_NAME}" -i ${CERT_FILE} -d dbm:"${certdir}"
      certutil -A -n "${CERT_NAME}" -t "TCu,Cu,Tu" -i ${CERT_FILE} -d dbm:"${certdir}"
  done
  find ~/ -name "cert9.db" -print0 | while read -r certDB
  do
      certdir=$(dirname "${certDB}");
      certutil -D -n "${CERT_NAME}" -i ${CERT_FILE} -d sql:"${certdir}"
      certutil -A -n "${CERT_NAME}" -t "TCu,Cu,Tu" -i ${CERT_FILE} -d sql:"${certdir}"
  done
  sudo mv ${CERT_FILE} /usr/local/share/ca-certificates/${CERT_FILE}.crt
  sudo update-ca-certificates
fi