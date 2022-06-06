#!/bin/sh
set -e

if [ -f /var/www/conf/nginx/default.conf ]; then
	sudo cp -f /var/www/conf/nginx/default.conf /etc/nginx/conf.d/
fi

if [ ! -f "/etc/nginx/certs/nginx.crt" ]; then
  sudo mkcert -key-file nginx.key -cert-file nginx.crt
  sudo mv nginx.key nginx.crt /etc/nginx/certs/
  sudo chown app:app /etc/nginx/certs/nginx.key /etc/nginx/certs/nginx.crt
fi

exec "$@"