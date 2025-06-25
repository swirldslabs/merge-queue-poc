#!/bin/bash
SOLO_CLUSTER_NAME="${SOLO_CLUSTER_NAME:-solo-e2e}"
MIRROR_NODE_VERSION="${MIRROR_NODE_VERSION:-0.131.0}"

images=(
  "ghcr.io/hashgraph/solo-containers/ubi8-init-java21:0.38.0"
  "busybox:1.36.1"
  "curlimages/curl:8.9.1"
  "docker.io/haproxytech/haproxy-alpine:2.4.25"
  "docker.io/envoyproxy/envoy:v1.21.1"
  "quay.io/minio/operator:v5.0.7"
  "ghcr.io/hiero-ledger/hiero-mirror-node-explorer/hiero-explorer:24.15.0"
  "ghcr.io/hiero-ledger/hiero-json-rpc-relay:0.67.0"
  "ghcr.io/hashgraph/solo-cheetah/cheetah:local"
  "quay.io/jcmoraisjr/haproxy-ingress:v0.14.5"
  "docker.io/bitnami/redis:7.4.2-debian-12-r6"
  "docker.io/bitnami/redis-sentinel:7.4.2-debian-12-r6"
  "gcr.io/mirrornode/hedera-mirror-grpc:${MIRROR_NODE_VERSION}"
  "gcr.io/mirrornode/hedera-mirror-importer:${MIRROR_NODE_VERSION}"
  "gcr.io/mirrornode/hedera-mirror-monitor:${MIRROR_NODE_VERSION}"
  "gcr.io/mirrornode/hedera-mirror-rest:${MIRROR_NODE_VERSION}"
  "gcr.io/mirrornode/hedera-mirror-rest-java:${MIRROR_NODE_VERSION}"
)

docker tag solo-cheetah ghcr.io/hashgraph/solo-cheetah/cheetah:local

for img in "${images[@]}"; do
  if ! docker image inspect "$img" > /dev/null 2>&1; then
    echo "Pulling $img..."
    docker pull "$img"
  else
    echo "Image $img already exists locally."
  fi
  kind load docker-image "$img" -n "${SOLO_CLUSTER_NAME}"
done