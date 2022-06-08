FROM nginx:1.18-alpine
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN apk update && \
    apk add --no-cache sudo curl nss-tools bash

RUN addgroup -g 1000 app && adduser -G app -g 1000 -u 1000 -h /var/www -s /bin/bash -S app
RUN touch /var/run/nginx.pid
RUN mkdir /sock
RUN chown -R app:app /var/cache/nginx/ /var/run/nginx.pid /sock

COPY ./conf/nginx.conf /etc/nginx/
COPY ./conf/default.conf /etc/nginx/conf.d/

RUN echo "app ALL=(ALL) NOPASSWD: /bin/cp -f /var/www/conf/nginx/default.conf /etc/nginx/conf.d/" >> /etc/sudoers.d/app

COPY ./docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]

RUN mkdir /var/www/html && chown -R app:app /var/www /var/www/html
VOLUME /var/www
WORKDIR /var/www/html
USER app:app

EXPOSE 8080

CMD [ "nginx", "-g", "daemon off;" ]