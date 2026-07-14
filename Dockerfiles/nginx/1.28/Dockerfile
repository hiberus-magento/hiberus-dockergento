FROM nginx:1.28-alpine
LABEL AUTHOR="Hiberus Magento <magento@hiberus.com>"

RUN apk update && \
    apk add --no-cache sudo curl nss-tools bash

RUN addgroup -g 1000 app && adduser -G app -g 1000 -u 1000 -h /var/www -s /bin/bash -S app
RUN touch /var/run/nginx.pid
RUN mkdir /sock
RUN chown -R app:app /var/cache/nginx/ /var/run/nginx.pid /sock

COPY --chmod=644 ./conf/nginx.conf /etc/nginx/
COPY --chmod=644 ./conf/default.conf /etc/nginx/conf.d/

RUN echo "app ALL=(ALL) NOPASSWD: /bin/cp -f /var/www/conf/nginx/default.conf /etc/nginx/conf.d/" >> /etc/sudoers.d/app

# --chmod=755 para que 'app' pueda leer+ejecutar el entrypoint (un chmod +x sobre
# un fichero 600 deja 711 y 'app' no puede leerlo -> "Permission denied").
COPY --chmod=755 ./docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]

RUN mkdir /var/www/html && chown -R app:app /var/www /var/www/html
VOLUME /var/www
WORKDIR /var/www/html
USER app:app

EXPOSE 8080

CMD [ "nginx", "-g", "daemon off;" ]