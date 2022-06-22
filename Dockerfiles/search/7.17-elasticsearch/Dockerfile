FROM alpine:3.16
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

ENV PATH /usr/share/elasticsearch/bin:$PATH
ENV ES_TMPDIR /tmp

RUN apk update && set -ex \ && apk add -t \
    .build-deps \
    su-exec \
    bash \
    openjdk11-jre-headless \
    wget \
    ca-certificates \
    gnupg \
    openssl

# Download Elastic Search
RUN wget -O /tmp/elasticsearch.tar.gz https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-7.17.3-no-jdk-linux-x86_64.tar.gz

# Install Elastic Search
RUN tar -xf /tmp/elasticsearch.tar.gz \
    && mv elasticsearch* /usr/share/elasticsearch \
    && adduser -D -h /usr/share/elasticsearch elasticsearch

# Create directories
RUN mkdir -p /usr/share/elasticsearch/data /usr/share/elasticsearch/logs /usr/share/elasticsearch/config /usr/share/elasticsearch/config/scripts /usr/share/elasticsearch/plugins \
    && chown -R elasticsearch:elasticsearch /usr/share/elasticsearch

# Remove x-pack-ml
RUN rm -rf /usr/share/elasticsearch/modules/x-pack-ml/platform/linux-x86_64

# Copy config files
COPY conf/elasticsearch.yml /usr/share/elasticsearch/config/elasticsearch.yml
COPY conf/logrotate /etc/logrotate.d/elasticsearch
COPY conf/log4j2.properties /usr/share/elasticsearch/config/log4j2.properties
COPY conf/jvm.options /usr/share/elasticsearch/config/jvm.options

ENV PATH /usr/share/elasticsearch/bin:$PATH
ENV ES_TMPDIR /tmp
ENV ES_JAVA_HOME /usr/lib/jvm/java-11-openjdk
ENV ES_JAVA_OPTS -Des.cgroups.hierarchy.override=/

# Install plugins
RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install --batch analysis-icu
RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install --batch analysis-phonetic

WORKDIR /usr/share/elasticsearch
VOLUME ["/usr/share/elasticsearch/data"]
USER elasticsearch
EXPOSE 9200 9300
CMD ["elasticsearch"]
