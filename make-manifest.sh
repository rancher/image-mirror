#!/bin/bash

# Usage: ./make-manifest.sh manifest-images-list destination-repo
# manifest-images-list file should be of the format
# source-repo:version comma-sep-os/arch

set -e

PARALLEL=${PARALLEL:-'false'}

info() {
  echo "$@" >&2
}

if [ "$#" -ne 2 ]; then
    info "... Need two arguments: image-mirror file and destination repo: $# $@"
    exit 1
fi

mirror_file=$1
dest_repo=$2

if ! command -v manifest-tool &>/dev/null; then
    info "Script requires github.com/estesp/manifest-tool for inspecting from gcr."
    exit 1
fi

lock() {
  local waiting=0
  while ! (set -o noclobber; >$1 ) 2>/dev/null; do
    [ ${waiting} = 0 ] && info "... Waiting for lock $1"; waiting=1
    sleep 1
  done
  info "... Obtained lock $1"
}
global_lock="${HOME}/.docker/mirror.lock"
#lock "${global_lock}"

workspace=$(mktemp -d)
cp ${mirror_file} ${workspace} 
cd "${workspace}"

cleanup() {
  local code=$?
  set +e
  trap - INT TERM EXIT
  info "... Cleanup code ${code}"
  rm -rf "${workspace}" "${global_lock}"
  [ $code -ne 0 ] && kill 0
  exit ${code}
}
trap cleanup INT TERM EXIT

cleanup-mirror() {
  local code=$?
  local mirror=$1
  local mirror_lock=$2
  set +e
  [ $code -ne 0 ] && info "!!! Failed mirror ${mirror}"
  rm -f "${mirror_lock}"
  exit ${code}
}

tr_repo() {
  (tr '/' '-' | tr ':' '-') <<<$1
}

check_if_img_with_tag_already_exists() {
    docker manifest inspect $1 > /dev/null 2>&1 ; echo $?
}

image_host_from_src() {
    arr=($(tr '/' ' '<<<$1))
    if [ "${#arr[@]}" -ge 3 ]; then
	echo "${arr[0]}"
    fi
}

image_from_src() {
    (rev | cut -d / -f1,2 | rev) <<<$1
}

create-mirror() {
  local image_host=$(image_host_from_src $1)
  local image=$(image_from_src $1)
  local repo=${image%:*}
  local version=${image#$repo:}
  local mirror="${ORG}/$(tr_repo ${repo}):${version}"
  local mirror_repo="${ORG}/$(tr_repo ${repo})"
  local mirror_lock="${workspace}/$(tr_repo ${mirror_repo}).lock"
  local manifest_amend

  # For each image/os/arch tuple, process the image, add necessary metadata
  # push the image to the destination and add it to the manifest.
  # Finally push the manifest to the destination
  process-manifest() {
    while read -r arch_img os arch variant; do
      local tag="${mirror_repo}:${version}-${arch}"
      local annotate_args="--os ${os} --arch ${arch}"
      local platform="${os}/${arch}"
      if [ -n "${variant}" ]; then
        tag+="-${variant}"
        annotate_args+=" --variant ${variant}"
        platform+="/${variant}"
      fi

      local img_name=""
      if [ -n "${image_host}" ]; then
	  img_name="${image_host}/"
      fi
      img_name+="${arch_img}"

      # Pull the image from the source if it is not already present in the destination repo
      # Otherwise get it from destination repo for addition to the manifest 
      tag-arch-img() {
	if [ $(check_if_img_with_tag_already_exists ${tag}) -ne 0 ]; then
	    info "... Pulling image from source repo: ${img_name}"
            docker pull -q ${img_name} >/dev/null
	    info "... Tagging original source repo image: ${tag}"
            docker tag ${img_name} ${tag} >/dev/null
	else
	    info "... Pulling preexisting image from destination repo with tags: ${tag}"
	    docker pull -q ${tag} >/dev/null
	fi
      }

      # If platform specific info is missing from the image, then manipulate JSON to add it
      set-image-platform() {
        info "... Set platform ${platform} for image ${tag}"
        mkdir -p ${tag}
        docker image save ${tag} -o ${tag}.tar >/dev/null
        for f in $(tar --list -f ${tag}.tar | grep -e '[./]json$'); do
          tar -C ${tag} -xf ${tag}.tar ${f}
          if jq '
              if has("os") then .os = "'${os}'" else . end |
              if has("architecture") then .architecture = "'${arch}'" else . end |
              if has("variant") then .variant = "'${variant}'" else . end
            ' <${tag}/${f} >${tag}/${f}.tmp 2>/dev/null; then
            mv -f ${tag}/${f}.tmp ${tag}/${f}
            tar -C ${tag} -uf ${tag}.tar ${f}
          fi
        done
        docker image load -q -i ${tag}.tar >/dev/null
        rm -rf ${tag}*
      }

      # If image is not present in the mirror, add it to the mirror
      # Create or amend existing manifest with the new image and add image specific tags to the manifest file
      annotate-manifest() {
	if [ $(check_if_img_with_tag_already_exists ${tag}) -ne 0 ]; then
	    info "... Pushing new image to destination repo with tags: ${tag}"      
	    docker push ${tag} >/dev/null	
	fi
        local digest=$(docker image inspect ${tag} | jq -r '.[] | .RepoDigests[0]')
	info "... Creating manifest: amend-option:${manifest_amend} manifest-name:${mirror} image:${digest}"
        docker manifest create ${manifest_amend} ${mirror} ${digest} >/dev/null
        docker manifest annotate ${mirror} ${digest} ${annotate_args} >/dev/null
        docker image rm -f ${tag} >/dev/null
        manifest_amend='--amend'
      }

      {
        tag-arch-img
        set-image-platform
        annotate-manifest
      }

    done < <(manifest-list ${image})
    info "--- Push mirror ${mirror}"
    docker manifest push ${mirror} >/dev/null
  }

  lock "${mirror_lock}"
  (
    trap 'cleanup-mirror ${mirror} ${mirror_lock}' EXIT
    info "+++ Create mirror ${mirror}"
    process-manifest    
  )
}

manifest-inspect() {
  # docker manifest inspect $1 | \
  #     jq -r '.manifests[] | "'$1'@\(.digest) \(.platform.os) \(.platform.architecture) \(.platform.variant // "")"'
  manifest-tool inspect --raw $1 2>/dev/null | \
      jq -r '.[] | 
        select((.Platform.architecture // "") != "") |
        "'$1'@\(.Digest) \(.Platform.os) \(.Platform.architecture) \(.Platform.variant // "")"
      '
}

manifest-template() {
  local image=$(image_from_src $1)
  local repo=${image%:*}
  local version=${image#$repo:}
  info "??? Generating manifest template for ${image}"
  for platform in $(tr ',' ' ' <<<${PLATFORMS}); do
    read -r os arch variant <<<$(tr '/' ' ' <<<${platform})
    local img=$(sed -e "
      s|REPO|${repo}|g;
      s|VERSION|${version}|g;
      s|OS|${os}|g;
      s|ARCH|${arch}|g;
      s|VARIANT|${variant}|g;
    " <<<${TEMPLATE})
    echo "${img} ${os} ${arch} ${variant}"
  done
}

manifest-list() {
  manifest=$(manifest-inspect $1)
  manifest=${manifest:-$(manifest-template $1)}
  echo "${manifest}"
}

create-mirrors() {
  info "+++ Create mirrors"
  local pids=()
  local code=0
  wait-pid() {
    wait $1 || code=$((code+1))
  }

  for mirror in $@; do
    [ -z "${mirror}" ] &&  continue
    create-mirror ${mirror} &
    local pid=$!
    if [[ "${PARALLEL}" = 'true' ]]; then
      pids+=(${pid})
    else
      wait-pid ${pid}
    fi
  done
  for pid in ${pids[@]}; do
    wait-pid ${pid}
  done

  info "--- Done create mirrors"
  if [ $code -ne 0 ]; then
    info "!!! Failed $code mirrors"
  fi
  return ${code}
}

clear-cache() {
  info "... Clear cache"
  rm -rf "${HOME}/.docker/manifests/"
  docker image prune -a -f >/dev/null
}

parse_file() {
  ORG=$dest_repo
  while IFS= read -r line;
  do
      params=( $line )
      repo_image_version=${params[0]}
      PLATFORMS=${params[1]}
      TEMPLATE='REPO-ARCH:VERSION'
      if [ "${#PLATFORMS[@]}" -eq 1 ]; then
	  TEMPLATE='REPO:VERSION'
      fi

      info "... Creating mirror for ${repo_image_version} at ${dest_repo} for platforms: ${PLATFORMS}"
      create-mirrors ${repo_image_version}
  done < $mirror_file
}


{
  clear-cache
  parse_file $@
}
