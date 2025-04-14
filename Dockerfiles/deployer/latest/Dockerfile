FROM php:8.2-fpm-buster
LABEL maintainer="Hiberus Magento <ddelgado@hiberus.com>"

RUN curl -o- https://deb.nodesource.com/setup_14.x | bash

RUN apt-get update && apt-get install -y --no-install-recommends \
	bash \
	nano \
	sudo \
	libpng-dev \
	libmcrypt-dev \
	libzip-dev \
	libicu-dev \
	libonig-dev \
	libxml2-dev \
	libxslt-dev \
	zip \
	unzip \
	libcurl4-openssl-dev \
	openssh-client \
	wget \
	rsync \
	git \
	rsync \
	&& rm -rf /var/lib/apt/lists/*

RUN docker-php-ext-install \
    bcmath \
    gd \
    intl \
    mbstring \
    pcntl \
    pdo_mysql \
    soap \
    xsl \
    zip \
    curl \
    sockets

RUN echo 'memory_limit = 2048M' >> /usr/local/etc/php/conf.d/docker-php-memlimit.ini

ENV COMPOSER_PROCESS_TIMEOUT=0

# Install composer
RUN curl -sS https://getcomposer.org/installer | \
  php -- --install-dir=/usr/local/bin --filename=composer

RUN curl -L -o /usr/local/bin/dep https://github.com/deployphp/deployer/releases/latest/download/deployer.phar
RUN chmod +x /usr/local/bin/dep

RUN mkdir /app
RUN groupadd -g 1000 app && useradd -g 1000 -u 1000 -d /app -s /bin/bash app
RUN echo "app ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/app

RUN mkdir -p /app/.ssh
RUN mkdir -p /app/.composer
RUN touch /app/.ssh/config /app/.ssh/id_rsa /app/.ssh/id_rsa.pub /app/.ssh/known_hosts
RUN chmod 600 /app/.ssh/id_rsa /app/.ssh/id_rsa.pub
RUN chmod 644 /app/.ssh/known_hosts

RUN chown -R app:app /app

USER app:app
VOLUME /app
WORKDIR /app

CMD ["php-fpm", "-R"]
