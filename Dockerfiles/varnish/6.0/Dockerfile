FROM varnish:6.6.1-alpine
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

ENV VARNISH_CONFIG /etc/varnish/default.vcl
ENV VARNISH_LISTEN :6081
ENV VARNISH_MANAGEMENT_LISTEN 127.0.0.1:6082
ENV VARNISH_STORAGE malloc,2048m

COPY config/varnish.vcl /etc/varnish/default.vcl
COPY docker-entrypoint.sh /docker-entrypoint.sh

EXPOSE 6081
EXPOSE 6082

ENTRYPOINT ["/bin/sh", "/docker-entrypoint.sh"]