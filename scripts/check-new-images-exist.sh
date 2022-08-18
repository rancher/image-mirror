#!/usr/bin/env bash
NEW_IMAGES=$(git diff -U0 HEAD~1 -- images-list | tail -n +5 | grep -v ^@@ | cut -d+ -f2 | awk '{ print $1":"$3 }')

if [ -z "$NEW_IMAGES" ]; then
  echo "Could not find new images from last commit"
  exit 0
fi

for NEW_IMAGE in $NEW_IMAGES; do
  echo "Checking if image ${NEW_IMAGE} exists"
  if ! skopeo inspect --retry-times=3 "docker://${NEW_IMAGE}" >/dev/null; then
    echo "Image ${NEW_IMAGE} does not exist"
    exit 1
  fi
  echo "Image ${NEW_IMAGE} exists"
done
