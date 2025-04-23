#!/bin/bash
# This script generates a specified total number of random file names with a given extension
# and creates empty files with those names in the target directory, followed by a delay between each batch.
# Usage: data/scripts/gen.sh <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>]

# Directory to store files
TARGET_DIR=$1
EXT=${2:-rcd_sig}
TOTAL=${3:-10}
DELAY=${4:-0.1}
BATCHES=${5:-10}

# Check if the target directory is provided and exists
if [[ -z "$TARGET_DIR" || ! -d "$TARGET_DIR" ]]; then
  echo "Usage: $0 <target_directory> [<file_extension>] [<total_number_of_files>] [<delay_in_seconds>] [<batches>]"
  echo "Please provide a valid directory."
  exit 1
fi

# Generate files in batches
for b in $(seq 1 $BATCHES); do
  echo "Batch: $b"
  for ((i = 0; i < $TOTAL; i++)); do
    n=$(uuidgen)
    file="${n}.${EXT}"
    echo "${file}"
    touch "${TARGET_DIR}/${file}"
  done
  echo "Generated $TOTAL files. Sleeping for ${DELAY}s..."
  sleep $DELAY
done