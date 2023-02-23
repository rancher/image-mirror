#!/usr/bin/env bash

if [ -z "$IMAGES" ]; then
  echo "IMAGES environment variable is not set, exiting"
  exit 1
fi

if [ -z "$TAGS" ]; then
  echo "TAGS environment variable is not set, exiting"
  exit 1
fi

for IMAGE in ${IMAGES//,/ }; do
  echo "Finding existing entry for image ${IMAGE}"
  if ! grep -q "^${IMAGE} " images-list; then
    echo "Unable to automatically add tag(s) for image ${IMAGE}, no existing entries found, exiting"
    exit 1
  fi
  MAPPING=$(grep "^${IMAGE} " images-list | awk '{ print $2 }' | uniq)
  MAPPING_COUNT=$(echo "${MAPPING}" | wc -l)
  echo "Found ${MAPPING_COUNT} mapping(s) for image ${IMAGE}"
  if [ $MAPPING_COUNT -eq 0 ]; then
    echo "Unable to automatically add tag(s) for image ${IMAGE}, no existing entries found, exiting"
    exit 1
  fi
  if [ $MAPPING_COUNT -gt 1 ]; then
    echo "More than one mapping found for image ${IMAGE}, checking for entry that contains '/mirrored-'"
    MIRRORED_MAPPING=$(echo "${MAPPING}" | grep \/mirrored\-)
    MIRRORED_MAPPING_COUNT=$(echo "${MIRRORED_MAPPING}" | wc -l)
    if [ $MIRRORED_MAPPING_COUNT -ne 1 ]; then
      echo "Unable to find mirrored mapping to use for image ${IMAGE}"
      echo "MIRRORED_MAPPING: ${MIRRORED_MAPPING}"
      echo "Unable to automatically add tag(s) for image ${IMAGE}, more than one mapping found (printed below), exiting"
      echo "$MAPPING"
      exit 1
    else
      echo "Found mirrored mapping to use for image ${IMAGE}"
      MAPPING=$MIRRORED_MAPPING
    fi
  fi
  echo "MAPPING for ${IMAGE} is ${MAPPING}"

  for TAG in ${TAGS//,/ }; do
    echo "Checking if tag ${TAG} already exists for image ${IMAGE}"
    if grep -q "${IMAGE} ${MAPPING} ${TAG}" images-list; then
      echo "Found existing tag ${TAG} for image ${IMAGE}, skipping"
      continue
    fi
    echo "Tag ${TAG} does not already exist for image ${IMAGE}"
    echo "Checking if image ${IMAGE}:${TAG} exists"
    if ! skopeo inspect --retry-times=3 "docker://${IMAGE}:${TAG}" >/dev/null; then
      echo "Image ${IMAGE}:${TAG} does not exist"
      exit 1
    fi
    echo "Image ${IMAGE}:${TAG} does exist"
    echo "Adding tag ${TAG} for image ${IMAGE}"
    echo "${IMAGE} ${MAPPING} ${TAG}" | tee -a images-list
  done
done

# Sort the file to satisfy CI
./scripts/sort-images-list.sh

echo "Done adding images and tags"
