#!/bin/sh
set -e
sudo chown -R app:app /var/www
if [ "${1#-}" != "$1" ]; then
	set -- php-fpm "$@"
fi
if [ "$COMPOSER_VERSION" ]; then
  sudo composer self-update --$COMPOSER_VERSION &> /dev/null
fi
exec "$@"