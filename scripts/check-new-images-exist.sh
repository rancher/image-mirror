#!/usr/bin/env bash
DIFF_CHECK="master"

if [ "${DRONE}" = "true" ]; then
  DIFF_CHECK="${DRONE_COMMIT_BEFORE} ${DRONE_COMMIT_AFTER}"
fi

echo "Checking for new images in commit(s) against ${DIFF_CHECK}"

NEW_IMAGES=$(git diff -U0 $DIFF_CHECK -- images-list | tail -n +5 | grep -v ^@@ | grep -v ^- | cut -d+ -f2 | awk '{ print $1":"$3 }')

if [ -z "${NEW_IMAGES}" ]; then
  echo "Could not find new images in commit(s) against ${DIFF_CHECK}"
  exit 0
fi

echo "Found new images in commit(s) against ${DIFF_CHECK}: ${NEW_IMAGES}"

for NEW_IMAGE in $NEW_IMAGES; do
  echo "Checking if image ${NEW_IMAGE} exists"
  if ! skopeo inspect --retry-times=3 "docker://${NEW_IMAGE}" >/dev/null; then
    echo "Image ${NEW_IMAGE} does not exist"
    exit 1
  fi
  echo "Image ${NEW_IMAGE} exists"
done
