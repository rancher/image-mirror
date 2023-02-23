#!/bin/bash

set -uo pipefail

function compare_image {
  SOURCE_SPEC="${1}"
  DEST_SPEC="${2}"
  TAG="${3}"

  trap 'echo -e "Failed checking image for ${DEST_SPEC}"' ERR

  # ensure that source specifies an explicit registry and repository
  IFS=/ read -a SOURCE <<< ${SOURCE_SPEC}
  if grep -vqE '[.:]|localhost' <<< ${SOURCE[0]}; then
    SOURCE=("docker.io" "${SOURCE[@]}")
  fi

  # recombine source spec
  printf -v SOURCE "/%s" "${SOURCE[@]}"; SOURCE=${SOURCE:1}

  # ensure that dest specifies an explicit registry and repository
  IFS=/ read -a DEST <<< ${DEST_SPEC}
  if grep -vqE '[.:]|localhost' <<< ${DEST[0]}; then
    DEST=("docker.io" "${DEST[@]}")
  fi

  # override destination org/user if set
  if [ ! -z "${DEST_ORG_OVERRIDE:-}" ]; then
    DEST[1]="${DEST_ORG_OVERRIDE}"
  fi

  # recombine dest spec
  printf -v DEST "/%s" "${DEST[@]}"; DEST=${DEST:1}

  echo
  echo "Comparing docker://${SOURCE}:${TAG} => docker://${DEST}:${TAG}"

  SOURCE_MANIFEST=$(skopeo inspect docker://${SOURCE}:${TAG} --raw)
  SOURCE_SCHEMAVERSION=$(jq -r '.schemaVersion' <<< ${SOURCE_MANIFEST})
  SOURCE_MEDIATYPE=$(jq -r '.mediaType' <<< ${SOURCE_MANIFEST})
 
  DEST_MANIFEST=$(skopeo inspect docker://${DEST}:${TAG} --raw)
  DEST_SCHEMAVERSION=$(jq -r '.schemaVersion' <<< ${DEST_MANIFEST})
  DEST_MEDIATYPE=$(jq -r '.mediaType' <<< ${DEST_MANIFEST})

  # Hard to compare different schema versions; skip for now
  if [ "${SOURCE_SCHEMAVERSION}" == "1" ] || [ "${DEST_SCHEMAVERSION}" == "1" ]; then
    echo -e "\tCan't compare legacy v1 schema"
    return
  fi

  # Source and dest are the same type - not complicated
  if [ "${SOURCE_MEDIATYPE}" == "$DEST_MEDIATYPE" ]; then
    # both are list - see which exist in destination
    if [ "${SOURCE_MEDIATYPE}" == "application/vnd.docker.distribution.manifest.list.v2+json" ]; then
      echo -e "\tSource: list\tDestination: list"
      DIGEST_ARCH_VARIANT_LIST=$(jq -r '.manifests | map(.digest + " " + .platform.architecture + " " + .platform.variant) | join("\n")' <<< ${SOURCE_MANIFEST})
      while read DIGEST ARCH VARIANT; do
        grep -qF "${DIGEST}" <<< ${DEST_MANIFEST} && RESULT="FOUND" || RESULT="NOT FOUND"
        echo -e "\tArchitecture: ${ARCH}\tVariant: ${VARIANT}\tDigest: ${DIGEST}\t${RESULT}"
      done <<< ${DIGEST_ARCH_VARIANT_LIST}
    # both are image - compare digests
    else
      DEST_CONFIG=$(skopeo inspect docker://${DEST}:${TAG} --raw --config)
      DEST_ARCH=$(jq -r '.architecture' <<< ${DEST_CONFIG})
      SOURCE_DIGEST='sha256:'$(echo -n "${SOURCE_MANIFEST}" | sha256sum | awk '{print $1}')
      DEST_DIGEST='sha256:'$(echo -n "${DEST_MANIFEST}" | sha256sum | awk '{print $1}')
      [ "${SOURCE_DIGEST}" == "${DEST_DIGEST}" ] && RESULT="MATCHED" || RESULT="NOT MATCHED - ${DEST_DIGEST}"
      echo -e "\tSource: image\tDestination: image"
      echo -e "\tArchitecture: ${DEST_ARCH}\tVariant: \tDigest: ${SOURCE_DIGEST}\t${RESULT}"
      diff -u <(echo -n "${SOURCE_MANIFEST}") <(echo -n "${DEST_MANIFEST}") || true
    fi
  # Either source or dest are list but not both; use ARCH from whichever one is an image and look it up in the list
  else
    # Source is list, dest is image
    if [ "${SOURCE_MEDIATYPE}" == "application/vnd.docker.distribution.manifest.list.v2+json" ]; then
      DEST_CONFIG=$(skopeo inspect docker://${DEST}:${TAG} --raw --config)
      DEST_ARCH=$(jq -r '.architecture' <<< ${DEST_CONFIG})
      DEST_DIGEST='sha256:'$(echo -n "${DEST_MANIFEST}" | sha256sum | awk '{print $1}')
      SOURCE_DIGEST=$(jq -r --arg DEST_ARCH "${DEST_ARCH}" '.manifests | map(select(.platform.architecture == $DEST_ARCH)) | map(.digest) | join("\n")' <<< ${SOURCE_MANIFEST})
      SOURCE_MANIFEST=$(skopeo inspect docker://${SOURCE}@${SOURCE_DIGEST} --raw)
      [ "${SOURCE_DIGEST}" == "${DEST_DIGEST}" ] && RESULT="MATCHED" || RESULT="NOT MATCHED - ${DEST_DIGEST}"
      echo -e "\tSource: list\tDestination: image"
      echo -e "\tArchitecture: ${DEST_ARCH}\tVariant: \tDigest: ${SOURCE_DIGEST}\t${RESULT}"
      diff -u <(echo -n "${SOURCE_MANIFEST}") <(echo -n "${DEST_MANIFEST}") || true
    # Source is image, dest is list
    else
      SOURCE_CONFIG=$(skopeo inspect docker://${SOURCE}:${TAG} --raw --config)
      SOURCE_ARCH=$(jq -r '.architecture' <<< ${SOURCE_CONFIG})
      SOURCE_DIGEST='sha256:'$(echo -n "${SOURCE_MANIFEST}" | sha256sum | awk '{print $1}')
      DEST_DIGEST=$(jq -r --arg SOURCE_ARCH "${SOURCE_ARCH}" '.manifests | map(select(.platform.architecture == $SOURCE_ARCH)) | map(.digest) | join("\n")' <<< ${DEST_MANIFEST})
      DEST_MANIFEST=$(skopeo inspect docker://${DEST}@${DEST_DIGEST} --raw)
      [ "${SOURCE_DIGEST}" == "${DEST_DIGEST}" ] && RESULT="MATCHED" || RESULT="NOT MATCHED - ${DEST_DIGEST}"
      echo -e "\tSource: image\tDestination: list"
      echo -e "\tArchitecture: ${SOURCE_ARCH}\tVariant: \tDigest: ${SOURCE_DIGEST}\t${RESULT}"
      diff -u <(echo -n "${SOURCE_MANIFEST}") <(echo -n "${DEST_MANIFEST}") || true
    fi
  fi
}

# Figure out if we should read input from a file or stdin
# If we're given a file, verify that it exists
if [ ! -z "${1:-}" ]; then
  INFILE="${1}"
  if [ ! -f "${INFILE}" ]; then
    echo "File ${INFILE} does not exist!"
    exit 1
  fi
else
  INFILE="/dev/stdin"
fi

echo "Reading SOURCE DESTINATION TAG from ${INFILE}"
while IFS= read -r LINE; do
  if grep -P '^(?!\s*(#|//))\S+\s+\S+\s+\S+' <<< ${LINE}; then
    compare_image ${LINE}
  fi
done < "${INFILE}"
