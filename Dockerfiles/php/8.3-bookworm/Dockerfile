FROM php:8.3-fpm-bookworm
LABEL AUTHOR="Hiberus Tecnologia <dabad@hiberus.com>"

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
    libonig-dev \
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

RUN docker-php-ext-configure gd --with-freetype --with-jpeg --with-webp
RUN docker-php-ext-configure ldap

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
    sockets \
    ftp \
    ldap

RUN pecl channel-update pecl.php.net \
    && pecl install xdebug-3.3.0 \
    && docker-php-ext-enable xdebug \
    && sed -i -e 's/^zend_extension/\;zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini

# Install composer
RUN curl -sS https://getcomposer.org/installer | \
  php -- --install-dir=/usr/local/bin --filename=composer

# Install NodeJS
RUN curl -sL https://deb.nodesource.com/setup_16.x | bash -
RUN apt-get update && apt-get install -y nodejs bzip2 && rm -rf /var/lib/apt/lists/*
RUN npm i -g grunt-cli

# Install Magento Cloud
RUN curl -sS https://accounts.magento.cloud/cli/installer | php
RUN mv /root/.magento-cloud /var/www/.magento-cloud
RUN ln -s /var/www/.magento-cloud/bin/magento-cloud /usr/bin/magento-cloud
RUN chmod -R 777 /var/www/.magento-cloud/bin/magento-cloud /usr/bin/magento-cloud

# Install N98-Magerun
RUN wget https://files.magerun.net/n98-magerun2.phar
RUN mv ./n98-magerun2.phar /usr/local/bin/n98-magerun
RUN chmod 777 /usr/local/bin/n98-magerun

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
