#!/bin/sh
set -e

if [ -f /var/www/conf/nginx/default.conf ]; then
	sudo cp -f /var/www/conf/nginx/default.conf /etc/nginx/conf.d/
fi

exec "$@"