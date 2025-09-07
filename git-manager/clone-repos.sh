#!/bin/bash

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 file with repo URLs"
    exit 1
fi

repo_file=$1
target_directory=$2


if [ ! -f "$repo_file" ]; then
    echo "File does not exist: $repo_file"
    exit 1
fi

if [ ! -d "$target_directory" ]; then
    echo "Directory does not exist: $target_directory"
    exit 1
fi

cd "$target_directory" || { echo "Failed to change directory to $target_directory"; exit 1; }

while IFS= read -r repo_url; do
    if [ -n "$repo_url" ]; then
        git clone "$repo_url"
    fi
done < "$repo_file"

echo "Cloning completed."
