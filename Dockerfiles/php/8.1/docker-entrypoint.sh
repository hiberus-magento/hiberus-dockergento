#!/bin/sh
set -e
sudo chown -R app:app /var/www
if [ "${1#-}" != "$1" ]; then
	set -- php-fpm "$@"
fi
if [ -n "$COMPOSER_VERSION" ]; then
  echo "Updating Composer to $COMPOSER_VERSION"
  sudo composer self-update "$COMPOSER_VERSION" || true
fi
exec "$@"