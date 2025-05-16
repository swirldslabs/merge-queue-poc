# Solo-Cheetah

`solo-cheetah` is a Go-based project designed for efficient file scanning, processing, and storage management. It supports both local and remote storage backends and provides a modular architecture for handling various file operations.

## Features
- File scanning with customizable patterns.
- S3, GCS, Local directory as storage support.
- Configurable pipelines for scanning and processing.
- Graceful shutdown handling.
- Pluggable storage backends.
- Profiling and metrics support.

---

## Prerequisites
- **Go**: Ensure Go is installed (version 1.20 or later).
- **Task**: Install [Task](https://taskfile.dev/) for task automation.

---

## Building the Project
To build the project using `Taskfile.yml`, follow these steps:

1. Clone the repository:
   ```bash
   git clone https://github.com/leninmehedy/solo-cheetah.git
   cd solo-cheetah
   ```

2. Build the project and Docker image:
   ```bash
   task build:image
   ```

This will compile the project and generate the executable in the `bin/` directory. It will also build a Docker image named `solo-cheetah` with the latest tag.

---

## Running the Project
To run the project, use the following commands:

1. Setup MinIO (in separate terminal):
```bash 
docker run --rm -p 9000:9000 -p 9001:9001 --name minio \
  -e "MINIO_ROOT_USER=solo-cheetah" \
  -e "MINIO_ROOT_PASSWORD=changeme" \
  minio/minio server /data --console-address ":9001"
```
This will start a MinIO server on `localhost:9000` and the console on `localhost:9001`.
You can access the MinIO console at `http://localhost:9001` using the credentials.

2. Set environment variables:
   ```bash
   export S3_ACCESS_KEY=solo-cheetah # Set your S3 access key
   export S3_SECRET_KEY=changeme     # Set your S3 secret key
   export GCS_ACCESS_KEY=solo-cheetah # Set your GCS access key
   export GCS_SECRET_KEY=changeme     # Set your GCS secret key
   ```

3. Run the Docker container:
   ```bash
   task run:image
   ```

This will start the application using the default configuration file located at `test/config/.cheetah/cheetah-container.yaml`.

---

## Configuration
`solo-cheetah` uses a configuration file to define pipelines, storage backends, and other settings. The configuration file can be specified using `--config` flag. Below is an example configuration (see the full example in `test/config/.cheetah/cheetah-container.yaml`):

```yaml
log:
  fileLogging: true
  level: Info
  directory: /app/logs
  fileName: cheetah.log
  maxSize: 1
  maxBackups: 10
  maxAge: 30
profiling:
    enabled: true
    interval: 5s
    directory: /app/stats
    enableServer: true
    serverHost: 0.0.0.0
    serverPort: 6060
    maxSize: 100 # in MB
pipelines:
  - name: record-stream-uploader
    scanner:
      path: /app/data/hgcapp/recordStreams
      pattern: ".rcd_sig" # only specify marker file extension starting with dot (e.g., ".rcd_sig", ".mf", ".evts_sig")
      recursive: true
      interval: 100ms
      batchSize: 1000
    processor: # each processor can upload to multiple storages concurrently or sequentially
      maxProcessors: 10
      fileExtensions: [ ".rcd", ".rcd_sig" ]
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
          bucket: cheetah 
          region: us-east-1
          prefix: /streams/record-streams
          endpoint: storage.googleapis.com
          accessKey: GCS_ACCESS_KEY # use this env variable
          secretKey: GCS_SECRET_KEY # use this env variable
          useSsl: true
        localDir: # not needed, used for dev/testing
          enabled: false
          path: /app/data/backup/recordStreams
          mode: 0755
```

### Storage Configuration
The `storage` section in the configuration defines the backends where files will be uploaded. Each backend can be enabled or disabled independently. Below are the supported storage backends:

#### 1. **S3 Storage**
```yaml
s3:
  enabled: true
  bucket: cheetah
  region: us-east-1
  prefix: streams/record-streams
  endpoint: localhost:9000
  accessKey: S3_ACCESS_KEY # use this env variable
  secretKey: S3_SECRET_KEY # use this env variable
  useSsl: false
```
- **enabled**: Whether to enable S3 storage.
- **bucket**: The S3 bucket name.
- **region**: The S3 region.
- **prefix**: The prefix (folder path) inside the bucket.
- **endpoint**: The S3 endpoint (useful for local testing with MinIO).
- **accessKey**: Environment variable for the S3 access key.
- **secretKey**: Environment variable for the S3 secret key.
- **useSsl**: Whether to use SSL for the connection.

#### 2. **GCS (Google Cloud Storage)**
```yaml
gcs:
  enabled: false
  bucket: lenin-test
  region: us-east-1
  prefix: cheetah/streams/record-streams
  endpoint: storage.googleapis.com
  accessKey: GCS_ACCESS_KEY # use this env variable
  secretKey: GCS_SECRET_KEY # use this env variable
  useSsl: true
```
- **enabled**: Whether to enable GCS storage.
- **bucket**: The GCS bucket name.
- **region**: The GCS region.
- **prefix**: The prefix (folder path) inside the bucket.
- **endpoint**: The GCS endpoint.
- **accessKey**: Environment variable for the GCS access key.
- **secretKey**: Environment variable for the GCS secret key.
- **useSsl**: Whether to use SSL for the connection.

#### 3. **Local Directory**
```yaml
localDir:
  enabled: false
  path: /app/data/backup/recordStreams
  mode: 0755
```
- **enabled**: Whether to enable local directory storage.
- **path**: The local directory path where files will be stored.
- **mode**: Directory permissions (e.g., `0755`).

---

## Taskfile Commands
Here are some useful `Taskfile` commands:

- **Build**: Compile the project.
  ```bash
  task build
  ```

- **Run**: Start the application.
  ```bash
  task run
  ```

- **Test**: Run all tests.
  ```bash
  task test
  ```

- **Lint**: Run linters.
  ```bash
  task lint
  ```

- **Build Docker Image**: Build the Docker image.
  ```bash
  task build:image
  ```

- **Run Docker Container**: Run the application in a Docker container.
  ```bash
  task run:image
  ```

---

## License
This project is licensed under the MIT License. See the `LICENSE` file for details.