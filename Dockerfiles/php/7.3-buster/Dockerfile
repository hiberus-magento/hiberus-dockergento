FROM php:7.3-fpm-buster
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    apt-utils \
    sendmail-bin \
    sendmail \
    msmtp \
    sudo \
    libbz2-dev \
    libjpeg62-turbo-dev \
    libpng-dev \
    libfreetype6-dev \
    libgeoip-dev \
    git \
    nano \
    wget \
    libgmp-dev \
    libgpgme11-dev \
    libmagickwand-dev \
    libmagickcore-dev \
    libicu-dev \
    libldap2-dev \
    libpspell-dev \
    libtidy-dev \
    libxslt1-dev \
    libyaml-dev \
    libzip-dev \
    libwebp-dev \
    zip \
    cron \
    gzip \
    libmcrypt-dev \
    lsof \
    mariadb-client \
    vim \
    procps \
    openssh-client \
    $PHPIZE_DEPS \
    && rm -rf /var/lib/apt/lists/*

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

# Install NodeJS
RUN curl -sL https://deb.nodesource.com/setup_12.x | bash -
RUN apt-get update && apt-get install -y nodejs bzip2
RUN npm i -g grunt-cli

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