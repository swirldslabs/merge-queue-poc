# Solo-Cheetah

`solo-cheetah` is a Go-based project for efficient file scanning, processing, and storage management.  
It supports local and remote storage backends and provides a modular architecture for handling various file operations.

## Features

- File scanning with customizable marker file extensions.
- S3, GCS, and local directory storage support.
- Configurable pipelines for scanning and processing.
- Graceful shutdown handling.
- Pluggable storage backends.
- Profiling and metrics support.

## Usage

Below is an example to run the application locally (macOS) using the published Docker image.

1. **Setup MinIO (in a separate terminal):**
   ```bash
   docker run --rm -p 9000:9000 -p 9001:9001 --name minio \
     -e "MINIO_ROOT_USER=solo-cheetah" \
     -e "MINIO_ROOT_PASSWORD=changeme" \
     minio/minio server /data --console-address ":9001"
   ```
This starts MinIO on `localhost:9000` and the console on `localhost:9001`.

2. **Create directories to mount:**
   ```bash
   mkdir -p /tmp/solo-cheetah/data
   mkdir -p /tmp/solo-cheetah/logs
   mkdir -p /tmp/solo-cheetah/stats
   ```

3. **Get a copy of `test/config/.cheetah/cheetah-container.yaml` from the repository.**  
   Modify as needed for your environment.

4. **Start the application:**
   ```bash
   export CHEETAH_VERSION=0.1.0-20250520 # Set the desired version
   docker run -it \
       -e S3_ENDPOINT=host.docker.internal:9000 \
       -e S3_ACCESS_KEY=solo-cheetah \
       -e S3_SECRET_KEY=changeme \
       -v $(PWD)/test/config:/app/config \
       -v /tmp/solo-cheetah/data:/app/data \
       -v /tmp/solo-cheetah/logs:/app/logs \
       -v /tmp/solo-cheetah/stats:/app/stats \
       ghcr.io/hashgraph/solo-cheetah/cheetah:"${CHEETAH_VERSION}" upload --config cheetah-container.yaml
   ```

5. **Generate test file pairs (.rcd and .rcd_sig) in `/tmp/solo-cheetah/data/hgcapp/recordStreams`:**
   ```bash
   mkdir -p /tmp/solo-cheetah/data/hgcapp/recordStreams
   echo "test" > /tmp/solo-cheetah/data/hgcapp/recordStreams/test.rcd
   echo "test" > /tmp/solo-cheetah/data/hgcapp/recordStreams/test.rcd_sig
   ```

---

# Development

## Prerequisites

- **Go**: v1.20 or later
- **Task**: Install [Task](https://taskfile.dev/) for automation

---

## Building the Project

1. Clone the repository:
   ```bash
   git clone https://github.com/leninmehedy/solo-cheetah.git
   cd solo-cheetah
   ```

2. Build the executables:
   ```bash
   task build
   ```

---

## Running the Project

1. (Optional) Start MinIO:
   ```bash
   task run:minio-local
   ```

2. Set environment variables:
   ```bash
   export S3_ENDPOINT=0.0.0.0:9000
   export S3_ACCESS_KEY=solo-cheetah
   export S3_SECRET_KEY=changeme
   export GCS_ENDPOINT=0.0.0.0:9000
   export GCS_ACCESS_KEY=solo-cheetah
   export GCS_SECRET_KEY=changeme
   ```

3. Run cheetah:
   ```bash
   task run -- upload --config test/config/.cheetah/cheetah-local.yaml
   ```

---

## Configuration

The configuration file defines pipelines, storage backends, and other settings.  
Specify it with the `--config` flag.

**Example: `test/config/.cheetah/cheetah-local.yaml`**
```yaml
log:
   fileLogging: true
   level: info
   directory: test/logs
   fileName: cheetah.log
   maxSize: 100 # in MB
   maxBackups: 10
   maxAge: 30
profiling:
   enabled: false
   interval: 5s
   directory: test/stats
   enableServer: true
   serverHost: 127.0.0.1
   serverPort: 6060
   maxSize: 100 # in MB
pipelines:
   - name: record-stream-uploader
     enabled: true
     stopOnError: true
     scanner:
        directory: /tmp/solo-cheetah/data/hgcapp/recordStreams #/tmp/solo-cheetah/data/hgcapp/recordStreams
        pattern: ".rcd_sig"
        interval: 100ms
        batchSize: 1000
     processor: # each processor can upload to multiple storages concurrently or sequentially
        maxProcessors: 30
        flushDelay: 100ms
        fileMatcherConfigs:
           - matcherType: basic # other types are sequential and glob
             patterns: [".rcd.gz", ".rcd_sig"] # derives names like {{.markerName}}.rcd.gz and {{.markerName}}.rcd_sig
           - matcherType: sequential
             patterns: ["sidecar/{{.markerName}}_##.gz"] # markerName is the name of the marker without the extension
        retry:
           limit: 5
        storage:
           s3:
              enabled: true
              bucket: cheetah
              region: us-east-1
              prefix: streams/record-streams
              endpoint: localhost:9000
              accessKey: S3_ACCESS_KEY # use this env variable
              secretKey: S3_SECRET_KEY # use this env variable
              useSsl: false
           gcs:
              enabled: false
              bucket: lenin-test
              region: us-east-1
              prefix: cheetah/streams/record-streams
              endpoint: storage.googleapis.com
              accessKey: GCS_ACCESS_KEY # use this env variable
              secretKey: GCS_SECRET_KEY # use this env variable
              useSsl: true
           localDir: # not needed, it is used for dev/testing
              enabled: false
              path: /tmp/solo-cheetah/data/backup/recordStreams
              mode: 0755
```

### Storage Configuration

Each backend can be enabled or disabled independently.

#### S3 Storage
```yaml
s3:
  enabled: true
  bucket: cheetah
  region: us-east-1
  prefix: streams/record-streams
  endpoint: localhost:9000
  accessKey: ${S3_ACCESS_KEY}
  secretKey: ${S3_SECRET_KEY}
  useSsl: false
```

#### GCS Storage
```yaml
gcs:
  enabled: false
  bucket: cheetah
  region: us-east-1
  prefix: streams/record-streams
  endpoint: storage.googleapis.com
  accessKey: ${GCS_ACCESS_KEY}
  secretKey: ${GCS_SECRET_KEY}
  useSsl: true
```

#### Local Directory
```yaml
localDir:
  enabled: false
  path: /app/data/backup/recordStreams
  mode: 0755
```
---
## Profiling
If profiling is enabled in the config, you can access the pprof profiling server at `http://localhost:6061/debug/pprof/` and snapshot profile at `http://localhost:6060/v1/last-snapshot`
```
# check heap profile 
go tool pprof -http=localhost:8081 http://localhost:6061/debug/pprof/heap

# check goroutine profile 
go tool pprof -http=localhost:8082 http://localhost:6061/debug/pprof/goroutine

# check custom snapshot profile (CPU and Memory)
curl -s -v http://localhost:6060/v1/last-snapshot
```
--- 

## Load into local cluster
To load the application into a local Kubernetes cluster, you can use the commands below. It will load the image with tag `ghcr.io/hashgraph/solo-cheetah/cheetah:local`. 
``` 
task build:image
SOLO_CLUSTER_NAME=solo task load
```
## Use a local docker proxy
It is recommended to use a local Docker registry proxy to speed up image pulls and cache images.

- Start the proxy server (Note: this doesn't yet work for gcr.io)
```sh
export DOCKER_REGISTRY_PROXY=localhost:3128
export DOCKER_MIRROR_CACHE=$HOME/docker_mirror_cache # change this to your preferred cache location
export DOCKER_MIRROR_CERTS=$HOME/docker_mirror_certs # change this to your preferred certs location
mkdir -p $DOCKER_MIRROR_CACHE $DOCKER_MIRROR_CERTS
docker run --rm --name docker_registry_proxy -it \
       --net kind --hostname docker-registry-proxy \
       -p 0.0.0.0:3128:3128 -e ENABLE_MANIFEST_CACHE=true \
       -e REGISTRIES="docker.io registry.k8s.io quay.io ghcr.io" \
       -v $DOCKER_MIRROR_CACHE:/docker_mirror_cache \
       -v $DOCKER_MIRROR_CERTS:/ca \
       rpardini/docker-registry-proxy:0.6.5
```
- Create your cluster if not already created e.g. `solo-e2e`
- Set docker proxy for your kind cluster
```sh
export SOLO_CLUSTER_NAME=solo-e2e # change this to your cluster name
./test/scripts/setup-docker-proxy.sh "${SOLO_CLUSTER_NAME}" 
```

## Taskfile Commands

- **Build:** `task build`
- **Run:** `task run`
- **Test:** `task test`
- **Lint:** `task lint`
- **Build Docker Image:** `task build:image`
- **Run Docker Container:** `task run:image`

---

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.