FROM nginx:1.18-alpine
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN apk update && \
    apk add --no-cache sudo curl nss-tools openssl bash

RUN addgroup -g 1000 app && adduser -G app -g 1000 -u 1000 -h /var/www -s /bin/bash -S app
RUN touch /var/run/nginx.pid
RUN mkdir /sock
RUN chown -R app:app /var/cache/nginx/ /var/run/nginx.pid /sock

# Prepare certs
RUN mkdir /etc/nginx/certs \
  && echo -e "\n\n\n\n\n\n\n" | openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /etc/nginx/certs/nginx.key -out /etc/nginx/certs/nginx.crt

RUN curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64" \
  && chmod +x mkcert-v*-linux-amd64 \
  && mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert \
  && chmod +r /etc/nginx/certs/*

COPY ./conf/nginx.conf /etc/nginx/
COPY ./conf/default.conf /etc/nginx/conf.d/

RUN echo "app ALL=(ALL) NOPASSWD: /bin/cp -f /var/www/conf/nginx/default.conf /etc/nginx/conf.d/" >> /etc/sudoers.d/app

COPY ./docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]

USER app:app
VOLUME /var/www
WORKDIR /var/www/html
EXPOSE 8843
EXPOSE 8000

CMD [ "nginx", "-g", "daemon off;" ]