#!/bin/bash
# This script generates a specified total number of random file names with a given extension
# and creates files of a specified size (default: 1MB) in the target directory, followed by a delay between each batch.
# Usage: data/scripts/gen.sh <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>] [<file_size_in_MB>]

# Directory to store files
TARGET_DIR=$1
MARKER_EXT=${2:-rcd_sig}
FILE_EXT=${3:-rcd}
TOTAL=${4:-10}
DELAY=${5:-0.1}
BATCHES=${6:-10}
FILE_SIZE_MB=${7:-1}

# Check if the target directory is provided and exists
if [[ -z "$TARGET_DIR" || ! -d "$TARGET_DIR" ]]; then
  echo "Usage: $0 <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>] [<file_size_in_MB>]"
  echo "Please provide a valid directory."
  exit 1
fi

# Generate files in batches
for b in $(seq 1 $BATCHES); do
  echo "Batch: $b"
  for ((i = 0; i < $TOTAL; i++)); do
    n=$(uuidgen)
    marker="${n}.${MARKER_EXT}"
    file="${n}.${FILE_EXT}"
    echo "Generating ${file} (${FILE_SIZE_MB}MB)..."
    dd if=/dev/urandom of="${TARGET_DIR}/${file}" bs=1M count=${FILE_SIZE_MB} status=none
    touch "${TARGET_DIR}/${marker}"
  done
  echo "Generated $TOTAL files. Sleeping for ${DELAY}s..."
  sleep $DELAY
done