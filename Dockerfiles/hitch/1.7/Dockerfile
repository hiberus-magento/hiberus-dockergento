FROM hitch:1.7
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"
USER hitch
CMD ["hitch", "--user=hitch", "--backend=[varnish]:6081", "--frontend=[*]:443", "/etc/hitch/testcert.pem"]