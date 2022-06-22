FROM opensearchproject/opensearch:1.2.4

ENV cluster.name "opensearch-cluster"
ENV node.name "opensearch-node1"
ENV discovery.seed_hosts "opensearch-node1,opensearch-node2"
ENV cluster.initial_master_nodes "opensearch-node1,opensearch-node2"
ENV bootstrap.memory_lock "true"
ENV OPENSEARCH_JAVA_OPTS "-Xms256m -Xmx256m"

COPY conf/jvm.options /usr/share/opensearch/config/jvm.options