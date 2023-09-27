#!/usr/bin/env bash

if [ -z "$FULL_IMAGES" ]; then
  echo "FULL_IMAGES environment variable is not set, exiting"
  exit 1
fi

for FULL_IMAGE in ${FULL_IMAGES//,/ }; do
  export IMAGES=$(echo "${FULL_IMAGE}" | awk -F: '{ print $1 }')
  export TAGS=$(echo "${FULL_IMAGE}" | awk -F: '{ print $2 }')
  echo "Running add-tag-to-existing-image.sh with IMAGES: ${IMAGES} and TAGS: ${TAGS}"
  bash scripts/add-tag-to-existing-image.sh
  RETVAL=$?
  if [ "${RETVAL}" -ne 0 ]; then
    exit $RETVAL
  fi
done

echo "Done running full image wrapper"
