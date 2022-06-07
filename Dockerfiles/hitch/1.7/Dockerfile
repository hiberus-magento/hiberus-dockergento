FROM hitch:1.7
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

RUN echo "ntp 6081/tcp" >> /etc/hosts
RUN echo "ntp 6081/tcp" >> /etc/hosts


CMD ["hitch", "--config=/etc/hitch/hitch.conf", "--backend=[varnish]:6081:/etc/hitch/certs/default"]