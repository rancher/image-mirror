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
  echo "Finding existing entry for repository ${IMAGE}"
  if ! grep -q "^${IMAGE} " images-list; then
    echo "Unable to automatically add tag(s) for repository ${IMAGE}, no existing entries found, exiting"
    exit 1
  fi
  MAPPING_COUNT=$(grep "^${IMAGE} " images-list | awk '{ print $2 }' | uniq | wc -l)
  echo "Found ${MAPPING_COUNT} mapping(s) for repository ${IMAGE}"
  if [ $MAPPING_COUNT -eq 0 ]; then
    echo "Unable to automatically add tag(s) for repository ${IMAGE}, no existing entries found, exiting"
    exit 1
  fi
  if [ $MAPPING_COUNT -gt 1 ]; then
    echo "$MAPPING"
    echo "Unable to automatically add tag(s) for repository ${IMAGE}, more than one mapping found, exiting"
    exit 1
  fi

  for TAG in ${TAGS//,/ }; do
    echo "Checking if tag ${TAG} already exists for repository ${IMAGE}"
    if grep -q "${IMAGE} ${MAPPING} ${TAG}" images-list; then
      echo "Found existing tag ${TAG} for repository ${IMAGE}, exiting"
      exit 1
    fi
    echo "Tag ${TAG} does not already exist for repository ${IMAGE}"
    echo "Checking if image ${IMAGE}:${TAG} exists"
    if ! skopeo inspect --retry-times=3 "docker://${IMAGE}:${TAG}" >/dev/null; then
      echo "Image ${IMAGE}:${TAG} does not exist"
      exit 1
    fi
    echo "Image ${IMAGE}:${TAG} does exist"
    echo "Adding tag ${TAG} for repository ${IMAGE}"
    echo "${IMAGE} ${MAPPING} ${TAG}" | tee -a images-list
  done
done

./scripts/sort-images-list.sh

echo "Done adding repositories and tags"
