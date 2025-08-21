#!/bin/bash
BIN_DIR=$(pwd)/bin
CONFIG_DIR=$(pwd)/test/config/.cheetah
DATA_DIR="/tmp/solo-cheetah/data/hgcapp"
LOG_DIR="$(pwd)/test/logs"
S3_ACCESS_KEY="solo-cheetah"
S3_SECRET_KEY="changeme"
S3_ENDPOINT="minio:9000"

docker run --rm --network=cheetah \
--memory "100m" \
--cpus "2" \
-v "${BIN_DIR}":/app/bin \
-v "${CONFIG_DIR}":/app/config \
-v "${DATA_DIR}":/app/data \
-v "${LOG_DIR}":/app/logs \
-p 6060:6060 \
-p 6061:6061 \
-e S3_ENDPOINT="${S3_ENDPOINT}" \
-e S3_ACCESS_KEY="${S3_ACCESS_KEY}" \
-e S3_SECRET_KEY="${S3_SECRET_KEY}" \
--name cheetah \
debian:bookworm-20250811-slim /app/bin/cheetah-linux-amd64 upload -c /app/config/s03.yaml
#ghcr.io/hashgraph/solo-cheetah/cheetah:local upload -c /app/config/s03.yaml;
# docker logs -f cheetah
