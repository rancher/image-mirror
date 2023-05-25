#!/usr/bin/env bash
DIFF_CHECK="master"

if [ "${DRONE}" = "true" ]; then
  DIFF_CHECK="${DRONE_COMMIT_BEFORE} ${DRONE_COMMIT_AFTER}"
fi

echo "Checking for new images in commit(s) against ${DIFF_CHECK}"

NEW_IMAGES=$(git diff -U0 $DIFF_CHECK -- images-list | tail -n +5 | grep -v ^@@ | grep -v ^- | cut -d+ -f2 | awk '{ print $1":"$3","$2 }')

if [ -z "${NEW_IMAGES}" ]; then
  echo "Could not find new images in commit(s) against ${DIFF_CHECK}"
  exit 0
fi

echo -e "Found new images in commit(s) against ${DIFF_CHECK}:\n${NEW_IMAGES}"

for NEW_IMAGE in $NEW_IMAGES; do
  SOURCE_IMAGE=$(echo "${NEW_IMAGE}" | cut -d, -f1)
  TARGET_IMAGE=$(echo "${NEW_IMAGE}" | cut -d, -f2)
  if [ $(awk '$2 == "'"$TARGET_IMAGE"'" { print $2 }' images-list | wc -l) -eq 1 ]; then
    echo "${TARGET_IMAGE} is a new target repository"
    TARGET_NAMESPACE=$(echo "${TARGET_IMAGE}" | cut -d/ -f1)
    TARGET_REPOSITORY=$(echo "${TARGET_IMAGE}" | cut -d/ -f2)
    echo "Checking if Docker Hub namespace ${TARGET_NAMESPACE} and repository ${TARGET_REPOSITORY} exists"
    if curl --silent --fail "https://hub.docker.com/v2/namespaces/${TARGET_NAMESPACE}/repositories/${TARGET_REPOSITORY}/" > /dev/null; then
      echo "OK: Docker Hub namespace ${TARGET_NAMESPACE} and repository ${TARGET_REPOSITORY} exists"
    else
      echo "ERROR: Docker Hub namespace ${TARGET_NAMESPACE} and repository ${TARGET_REPOSITORY} does not exist"
      exit 1
    fi
  else
    echo "${TARGET_IMAGE} is not a new target repository"
  fi
  echo "Checking if image ${SOURCE_IMAGE} exists"
  if ! skopeo inspect --retry-times=3 "docker://${SOURCE_IMAGE}" >/dev/null; then
    echo "ERROR: Image ${SOURCE_IMAGE} does not exist"
    exit 1
  fi
  echo "OK: Image ${SOURCE_IMAGE} exists"
done
