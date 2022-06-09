FROM hitch:1.7
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN apt-get update && apt-get -y install openssl curl

RUN curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64" \
  && chmod +x mkcert-v*-linux-amd64 \
  && mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert

RUN chown hitch /etc/hitch/testcert.pem
USER hitch

CMD ["hitch", "--user=hitch", "--backend=[varnish]:6081", "--frontend=[*]:443", "/etc/hitch/testcert.pem"]