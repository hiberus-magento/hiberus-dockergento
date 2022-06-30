FROM php:7.2-fpm-alpine
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN apk update && apk add \
    bash \
    apk-cron \
    git \
    gzip \
    libmcrypt-dev \
    libpng-dev \
    libwebp-dev \
    libzip-dev \
    msmtp \
    lsof \
    mariadb-client \
    vim \
    zip \
    bzip2 \
    procps \
    sudo \
    openssh-client \
	curl \
	g++ \
	freetype-dev \
	icu \
	icu-dev \
	libjpeg-turbo-dev \
	libxml2-dev \
	libxslt-dev \
	shadow \
	zlib-dev \
    oniguruma-dev \
    $PHPIZE_DEPS

RUN docker-php-ext-configure gd --with-freetype-dir=/usr/include/ --with-jpeg-dir=/usr/include/ --with-webp-dir=/usr/include/

RUN docker-php-ext-install \
    bcmath \
    gd \
    intl \
    mbstring \
    opcache \
    pcntl \
    pdo_mysql \
    soap \
    xsl \
    zip \
    sockets

RUN pecl channel-update pecl.php.net \
    && pecl install xdebug-2.9.8 \
    && docker-php-ext-enable xdebug \
    && sed -i -e 's/^zend_extension/\;zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini

# Install composer
RUN php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');" \
    && php composer-setup.php --install-dir /usr/local/bin --filename=composer \
    && php -r "unlink('composer-setup.php');" \
    && composer self-update --2

# Install NodeJS and grunt-cli
RUN apk add --update nodejs npm && npm i -g grunt-cli

RUN groupadd -g 1000 app && useradd -g 1000 -u 1000 -d /var/www -s /bin/bash app

COPY conf/www.conf /usr/local/etc/php-fpm.d/
COPY conf/php.ini /usr/local/etc/php/
COPY conf/xdebug.ini /usr/local/etc/php/conf.d/
COPY conf/php-fpm.conf /usr/local/etc/
COPY conf/php-msmtp.conf /etc/msmtp/msmtp.conf
COPY conf/php-mailhog.ini /usr/local/etc/php/conf.d/php-mailhog.ini

RUN mkdir /sock
RUN chown -R app:app /usr/local/etc/php/conf.d /sock
RUN mkdir -p /var/www && chown -R app:app /var/www/
RUN echo "app ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/app

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]

USER app:app
VOLUME /var/www
WORKDIR /var/www/html

CMD ["php-fpm", "-R"]