FROM hitch:1.7
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN apt-get update && apt-get -y install openssl

RUN chown hitch /etc/hitch/testcert.pem
USER hitch

CMD ["hitch", "--user=hitch", "--backend=[varnish]:6081", "--frontend=[*]:443", "--ciphers=EECDH+AESGCM:EDH+AESGCM:AES256+EECDH:AES256+EDH", "/etc/hitch/testcert.pem"]