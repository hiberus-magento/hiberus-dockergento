FROM elasticsearch:7.17.4
MAINTAINER "Hiberus Tecnologia <dabad@hiberus.com>"

# Copy config files
COPY conf/jvm.options /usr/share/elasticsearch/config/jvm.options

# Install plugins
RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install --batch analysis-icu
RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install --batch analysis-phonetic

EXPOSE 9200 9300
CMD ["elasticsearch"]
