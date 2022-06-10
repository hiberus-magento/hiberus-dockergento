FROM elasticsearch:5.6

RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install \
  analysis-icu

RUN /usr/share/elasticsearch/bin/elasticsearch-plugin install \
  analysis-phonetic

COPY conf/jvm.options /usr/share/elasticsearch/config/jvm.options