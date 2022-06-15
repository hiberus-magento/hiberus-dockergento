FROM hitch:1.7
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"
RUN chown hitch /etc/hitch/testcert.pem
USER hitch
CMD ["hitch", "--user=hitch", "--backend=[varnish]:6081", "--frontend=[*]:443", "/etc/hitch/testcert.pem"]