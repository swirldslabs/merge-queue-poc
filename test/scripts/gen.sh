#!/bin/bash
# This script generates a specified total number of random file names with a given extension
# and creates files of a specified size (default: 100KB) in the target directory, followed by a delay between each batch.
# Usage: data/scripts/gen.sh <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>] [<file_size_in_KB>]

# Directory to store files
TARGET_DIR=$1
MARKER_EXT=${2:-rcd_sig}
FILE_EXT=${3:-rcd.gz}
TOTAL=${4:-10}
DELAY=${5:-0.1}
BATCHES=${6:-10}
FILE_SIZE_KB=${7:-100}

# Check if the target directory is provided and exists
if [[ -z "$TARGET_DIR" || ! -d "$TARGET_DIR" ]]; then
  echo "Usage: $0 <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>] [<file_size_in_KB>]"
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
    echo "Generating ${file} (${FILE_SIZE_KB}KB)..."
    dd if=/dev/zero of="${TARGET_DIR}/${file}" bs=1K count=${FILE_SIZE_KB} status=none
    touch "${TARGET_DIR}/${marker}"
  done
  echo "Generated $TOTAL files [batch: $i/$BATCHES]. Sleeping for ${DELAY}s..."
  sleep $DELAY
done