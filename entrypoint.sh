#!/bin/bash

set -uo pipefail
BASEDIR=$(dirname $0)

if [ ! -z "${DOCKER_USERNAME:-}" ] && [ ! -z "${DOCKER_PASSWORD:-}" ]; then
  echo "Logging in to ${DOCKER_REGISTRY:-docker.io} as ${DOCKER_USERNAME}"
  docker login ${DOCKER_REGISTRY:-docker.io} --username=${DOCKER_USERNAME} --password-stdin <<< ${DOCKER_PASSWORD}
  export DOCKER_TOKEN=$(curl -s -d @- -X POST -H "Content-Type: application/json" https://hub.docker.com/v2/users/login/ <<< '{"username": "'${DOCKER_USERNAME}'", "password": "'${DOCKER_PASSWORD}'"}' | jq -r '.token')
fi

exec ${BASEDIR}/image-mirror.sh $@
