#!/bin/bash
# Instructions: Add the new images at the end of the images-list file, then run this
# script to generate a new images-list file with all images sorted in the correct order

set -e

HEADER="# This list should be sorted in lexicographical (alphabetical) order. Do not change existing entries. Comments lines may be started with # or //.\n# This list was reset in order to prevent breakage when moving to multiarch mirroring. See the following PR for more information:\n# https://github.com/rancher/image-mirror/pull/27\n#\n# LIST FORMAT: <SOURCE> <DESTINATION> <TAG>"

if [[ -f images-list ]]; then
    SORTED=$(grep -vE '^\s*(#|//)' images-list | sort -V)
    echo -e "${HEADER}\n${SORTED}" > images-list
fi
