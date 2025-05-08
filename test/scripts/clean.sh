#!/bin/bash

# Directory to delete files from
TARGET_DIR=$1
TOTAL=${2:-10}
DELAY=${3:-0.1}

# Check if the directory is provided and exists
if [[ -z "$TARGET_DIR" || ! -d "$TARGET_DIR" ]]; then
  echo "Usage: $0 <directory>"
  echo "Please provide a valid directory."
  exit 1
fi

# Delete 10 files at a time
while true; do
  # Find up to 10 files in the directory
  FILES=($(find "$TARGET_DIR" -maxdepth 1 -type f | head -n $TOTAL))

  # Break the loop if no files are found
  if [[ ${#FILES[@]} -eq 0 ]]; then
    echo "No more files to delete. Sleeping for 5s..."
    sleep 5
    continue
  fi

  # Delete the files
  for FILE in "${FILES[@]}"; do
    echo "Deleting: $FILE"
    rm -f "$FILE"
  done

  echo "Deleted 10 files. Continuing..."
  sleep $DELAY # Optional delay between deletions
done