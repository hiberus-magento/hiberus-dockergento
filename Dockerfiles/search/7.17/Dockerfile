FROM elasticsearch:7.17.3

RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install \
  analysis-icu \
  analysis-phonetic

COPY conf/jvm.options /usr/share/elasticsearch/config/jvm.options