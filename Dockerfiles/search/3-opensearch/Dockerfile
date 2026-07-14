FROM opensearchproject/opensearch:3.1.0

# Heap y opciones JVM se pasan por OPENSEARCH_JAVA_OPTS (la imagen las respeta).
# IMPORTANTE: NO sobrescribir config/jvm.options. En OpenSearch 3.x ese fichero
# incluye la línea `-javaagent:.../opensearch-agent-<ver>.jar` necesaria para el
# nuevo agente de seguridad (sustituye al SecurityManager de Java, eliminado en
# JDK reciente). Reemplazarlo provoca al arrancar:
#   NoClassDefFoundError: org.opensearch.javaagent.bootstrap.AgentPolicy$AnyCanExit
# -XX:UseSVE=0 silencia el warning de SVE en arm64 (Apple Silicon), es inofensivo.
ENV OPENSEARCH_JAVA_OPTS="-Xms512m -Xmx512m -XX:UseSVE=0"
ENV DISABLE_INSTALL_DEMO_CONFIG="true"
ENV DISABLE_SECURITY_PLUGIN="true"

CMD ["opensearch"]
