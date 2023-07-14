FROM php:7.4-fpm-alpine
LABEL maintainer="Hiberus Magento <ddelgado@hiberus.com>"

RUN apk update && apk add --no-cache \
	bash \
	nano \
	sudo \
	libzip-dev \
	zip \
	curl-dev \
	curl

RUN docker-php-ext-install \
    pdo_mysql

RUN echo 'memory_limit = 2048M' >> /usr/local/etc/php/conf.d/docker-php-memlimit.ini

RUN curl -L -o /usr/local/bin/masquerade https://github.com/elgentos/masquerade/releases/latest/download/masquerade.phar
RUN chmod +x /usr/local/bin/masquerade

RUN mkdir /app
RUN addgroup -g 1000 app && adduser -h /app -s /bin/bash -G app -u 1000 --disabled-password app
RUN echo "app ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/app
RUN chown -R app:app /app

COPY ./conf /app/config

USER app:app
VOLUME /app
WORKDIR /app