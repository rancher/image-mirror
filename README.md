# Mirroring External Images into Rancher Repo on Dockerhub

This repo is dedicated to mirror images from other organizations into Rancher.
There are no packaging changes or changes in the layers of these images.

## Mirroring Types

There are 2 types of images that are mirrored. 

### 1. Single-Arch Images

This list is maintained in the `images-list` file, which is structured with the following format...

```
<original-image-name> <rancher-image-name> <image-tag>
```

The basic `rancher-image-name` structure is `mirrored-<org>-<repo>` and here is an example...
```
banzaicloud/logging-operator rancher/mirrored-banzaicloud-logging-operator 3.7.0
```

### 2. Multi-Arch Images

Support has been recently added for multi-arch images.
Learn more [here](https://github.com/rancher/image-mirror/pull/27).

## Adding New Images

When adding new images to the repo, please indicate so in the pull request.

An EIO team member or manager will need to create the repo in DockerHub as well as add the `automatedcipublisher` as a team member in DockerHub with `write` access in order for the images to be automatically pushed.

## Updating Existing Images

**Do not** update the tag in the `images-list` file for an updated image to be pulled/pushed. Add an additional entry with the new tag.

## Images Requiring Multi-Arch support

- Flannel
- CoreDNS
- Longhorn
