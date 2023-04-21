# Mirroring External Images into Rancher Repo on Dockerhub

This repo is dedicated to mirror images from other organizations into Rancher.
There are no packaging changes or changes in the layers of these images.

## Mirroring images

The list is maintained in the `images-list` file, which is structured with the following format...

```
<original-image-name> <rancher-image-name> <image-tag>
```

The basic `rancher-image-name` structure is `mirrored-<org>-<repo>` and here is an example...
```
banzaicloud/logging-operator rancher/mirrored-banzaicloud-logging-operator 3.7.0
```

Images are mirrored using the `scripts/image-mirror.sh` script.

## Adding New Images

When adding new images to the repo, please indicate so in the pull request.

An EIO team member or manager will need to create the repo in DockerHub as well as add the `automatedcipublisher` as a team member in DockerHub with `write` access in order for the images to be automatically pushed.

## Updating Existing Images

**Do not** update the tag in the `images-list` file for an updated image to be pulled/pushed. Add an additional entry with the new tag.

## Adding new tags to existing images

### Scheduled

There is also a scheduled workflow called [Retrieve image tags](https://github.com/rancher/image-mirror/actions/workflows/retrieve-image-tags.yml) that can be used if you have images that needs new tags added automatically. It will check a configurable source for available tags, and use the found tags to dispatch the workflow [Add tag to existing image
](https://github.com/rancher/image-mirror/actions/workflows/add-tag-to-existing-image.yml). The configuration lives in [`config.json`](https://github.com/rancher/image-mirror/actions/workflows/retrieve-image-tags/config.json). The basic structure is having a descriptive key (pick your own), and specify the list of images for which the available tag(s) need to be looked up (`versionSource`), and an optional SemVer constraint if you need to limit what tags are used. The current datasources are:

- `github-releases`: This will use GitHub releases as source, excluding pre-releases. This can be used if you need to keep all tags from the configured images in sync with GitHub releases
- `github-latest-release`: This will use the release on GitHub marked as `Latest release`. This can be used if you only want one release to be added that is marked as latest.
- `registry`: This will use the registry of the first image and look up available tags.

The current filters for tags are:

- `versionConstraint`: This is a semver constraint that will match the given expression and ignore tags that do not match.
- `versionFilter`: This is a regex filter that will match the given expression and ignore tags that do not match.

See an example configuration below:

```
{
  "vsphere-cpi": {
    "images": [
      "gcr.io/cloud-provider-vsphere/cpi/release/manager"
    ],
    "versionSource": "github-releases:kubernetes/cloud-provider-vsphere",
    "versionConstraint": ">1.21.0"
  },
  "flannel": {
    "images": [
      "flannel/flannel"
    ],
    "versionSource": "github-latest-release:flannel-io/flannel"
  },
  "bci-busybox": {
    "images": [
      "registry.suse.com/bci/bci-busybox"
    ],
    "versionSource": "registry",
    "versionFilter": "^15.4.",
    "latest": "true"
  },
  "skopeo": {
    "images": [
      "quay.io/skopeo/stable"
    ],
    "versionSource": "registry",
    "versionFilter": "^v1.\\d{2}.\\d+$",
    "latest": "true"
  },
  "pause": {
    "images": [
      "registry.k8s.io/pause"
    ],
    "versionSource": "registry",
    "versionFilter": "^3.\\d+$",
    "latest": "true"
  },
  "epinio": {
    "images": [
      "ghcr.io/epinio/epinio-server"
    ],
    "versionSource": "registry",
    "versionFilter": "^v1.\\d+.\\d+$",
    "latest": "true"
  },
  "csi-release-syncer": {
    "images": [
      "gcr.io/cloud-provider-vsphere/csi/release/syncer"
    ],
    "versionSource": "registry",
    "versionFilter": "^v2.\\d+.\\d+$",
    "latest": "true"
  }
}
```

If you want to manually test your configuration changes to check if the correct tags are found, you can use the following commands depending on your available runtime:

#### Docker 

```
docker run -v $PWD:/code -w /code/retrieve-image-tags python:3.10 bash -c "pip install -qr requirements.txt && python retrieve-image-tags.py"
```

#### podman

```
podman run -v $PWD:/code -w /code/retrieve-image-tags python:3.10 bash -c "pip install -qr requirements.txt && python retrieve-image-tags.py"
```

#### containerd

```
ctr images pull docker.io/library/python:3.10
ctr run -t --net-host --mount type=bind,src=$PWD,dst=/code,options=rbind:ro --cwd /code/retrieve-image-tags --rm docker.io/library/python:3.10 workflow-test bash -c "pip install -qr requirements.txt && python retrieve-image-tags.py"
```

### Using scripts

You can use the following commands/scripts to add a tag to an **existing** image. Make sure the `IMAGES` environment variable is set to the image(s) you want to add a tag to, and the `TAGS` environment variable is set to the tags you want to add to the images. The script will check:

- If the image provided is already existing, else it will fail because it only supports adding tags to existing images.
- If there is only one mapping in `images-list`, else it will fail because it cannot determine what mapping to use.
- If the tag for the image is not already present, else it will fail because it is not new.
- If the tag for the image exists, else it will fail as it cannot be mirrored.

After everything is successfull, it will add the tag to `images-list`. If all images and tags are added, it will sort `images-list`.

See an example below:

```
IMAGES=quay.io/coreos/etcd TAGS=v3.4.20 make add-tag-to-existing-image.sh
```

Optionally, you can also check if the newly added image tag exists (this will also be run in Drone):

```
make check-new-images-exist.sh
```

### Using GitHub Actions workflow

You can use the [Add tag to existing image](https://github.com/rancher/image-mirror/actions/workflows/add-tag-to-existing-image.yml) workflow to provide a comma separated list of existing images and to be added tags, and it will create a pull request automatically with the changes. See [Using scripts](#using-scripts) what this does in detail.

Example inputs:

```
Images: quay.io/cilium/cilium,quay.io/cilium/operator-aws,quay.io/cilium/operator-azure,quay.io/cilium/operator-generic 
Tags: v1.12.1
```
