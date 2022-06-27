FROM opensearchproject/opensearch:1.2.4

ENV OPENSEARCH_JAVA_OPTS "-Xms256m -Xmx256m"
ENV DISABLE_INSTALL_DEMO_CONFIG "true"
ENV DISABLE_SECURITY_PLUGIN "true"

COPY conf/jvm.options /usr/share/opensearch/config/jvm.options

CMD ["opensearch"]