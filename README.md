# Rancher Image Mirror

This repo is dedicated to mirroring images from other organizations to the
places they need to be for Rancher and its associated projects to use them.

## Overview

- `regsync` does the actual mirroring. `regsync` is configured by `regsync.yaml`,
  and it runs in a workflow upon merges to `master`.
- `regsync.yaml` is generated from `config.yaml` using Go code in `tools/`.
  `config.yaml` is what a user typically interacts directly with.
- `autoupdate.yaml` may be used to configure autoupdates for your images in
  `config.yaml`. The autoupdate workflow runs daily, and creates pull requests
  that update `config.yaml`, and by extension, `regsync.yaml`.

A typical workflow looks like this:
1. Modify `config.yaml` with your desired changes. See below for documentation
   on its fields.
2. Run `scripts/build-tools.sh` to build `image-mirror-tools`.
3. Run `bin/image-mirror-tools generate-regsync` to update `regsync.yaml` from
   the new `config.yaml`.
4. Run `bin/image-mirror-tools format` to format any files that might need it.
5. Run `bin/image-mirror-tools validate` to catch certain errors you may have
   made.
6. Make a pull request to get your changes merged.

### Adding New Images

When mirroring new images via this repo, please indicate so in the pull request.
You will need to submit a request to EIO, who will create the repo in
DockerHub. If this is not done, the mirroring workflow will fail, potentially
inconveniencing other users. This only applies to DockerHub; nothing special
needs to be done for mirroring a new image to the Rancher Prime registry.

### Image Prefixes

Every image that is mirrored by this repository should have a prefix that
communicates some information about it. The only exception is legacy images:
in the past the Rancher project did not add prefixes to mirrored images.
All new images must have a prefix.

| Prefix | Meaning |
| ------------- | ------------- |
| `mirrored-` | The image was mirrored from somewhere else. This is the default choice, and is used when the below prefixes do not apply.
| `appco-` | The image originated in the [SUSE Application Collection](https://apps.rancher.io/). We use images from the application collection because they have a better security posture.
| `hardened-` | The image has been hardened. This repository does not concern itself with hardened images.

## File Purpose/Structure

### `regsync.yaml`

`regsync.yaml` is mostly for use by `regsync`. It is generated from `config.yaml`,
and is not very easy to read. It should never be modified directly. It can,
however, be useful for checking that `config.yaml` changes will have the expected
effect on mirroring.

### `regsync-daily.yaml`

`regsync-daily.yaml` is a special case for images that must be mirrored daily.
Avoid using it if possible - it does not get you the benefits of using `config.yaml`
and `image-mirror-tools`. There should be a very good reason for using it, if you
do. `regsync-daily.yaml` is managed manually, and not touched by any automation.

As of the time of writing, `regsync-daily.yaml` is used only for Neuvector images.
The Neuvector images use it because the daily image builds incorporate the latest CVE
information. We must mirror the `latest` tag of these images each day for this
information to be available to users.

### `config.yaml`

#### `Registries`

`Registries` describes the registries that image-mirror interfaces with.
This section roughly correlates to the `creds` section of `regsync.yaml`.

| Field | Required | Description |
| ------------- | ------------- |------------- |
| `BaseUrl` | yes | The base URL for the registry. Appending `/` plus an image name should be a valid image reference.
| `Password` | yes | The password to use when authenticating against the registry. See [the regsync documentation](https://regclient.org/usage/regsync/) for more details.
| `Registry` | yes | The registry URL. See [the regsync documentation](https://regclient.org/usage/regsync/) for more details.
| `ReqConcurrent` | no | The number of concurrent requests that are made to this registry. See [the regsync documentation](https://regclient.org/usage/regsync/) for more details.
| `DefaultTarget` | no | Whether the Registry is used as a target registry for a given Image when the `TargetRegistries` field of the Image is not set.
| `Username` | yes | The username to use when authenticating against the registry. See [the regsync documentation](https://regclient.org/usage/regsync/) for more details.

#### `Images`

`Images` describes the images that we want to mirror to each target registry.

| Field | Required | Description |
| ------------- | ------------- |------------- |
| `DoNotMirror` | no | Set to `true` to exclude the entire image from regsync.yaml. Alternatively, set to an array of strings to specify tags to exclude from regsync.yaml.
| `SourceImage` | yes | The source image. If there is no host, the image is assumed to be from Docker Hub.
| `Tags`        | yes | The tags to mirror.
| `TargetImageName` | no | By default, the target image name is derived from the source image, and is of the format `mirrored-<org>-<name>`. For example, `banzaicloud/logging-operator` becomes `mirrored-banzaicloud-logging-operator`. However, there are some images that do not follow this convention - this field exists for these cases. New images should not set this field.
| `TargetRegistries` | no | Registries to mirror the image to. Registries are specified via their `BaseUrl` field. If not specified, the Image is mirrored to all Registries that have `DefaultTarget` set to true.

### `autoupdate.yaml`

`autoupdate.yaml` defines configuration for automatically updating image tags
based on various update strategies that monitor sources for new tags. Each
entry specifies a strategy for finding tags of images to potentially add to
`config.yaml`, which are then submitted as pull requests.

| Field           | Required | Description |
|-----------------| ------------- |------------- |
| `Name`          | yes | A unique identifier for this autoupdate entry. Used for logging and generating branch names for pull requests.
| `GithubRelease` | no | See [`GithubRelease`](#githubrelease).
| `HelmLatest`    | no | See [`HelmLatest`](#helmlatest).
| `Registry`      | no | See [`Registry`](#registry).
| `Reviewers`     | yes | A list of GitHub users or teams that own the autoupdate entry. Teams should be in the format `org/team-slug`.

#### `GithubRelease`

The `GithubRelease` strategy fetches all release tags that matches the `VersionConstraint` from a GitHub
repository and applies it to the specified images.
If `LatestOnly` is true, it only fetches from the latest release and does not consider the `VersionConstraint`.

| Field               | Required | Description |
|---------------------|----------|------------- |
| `Owner`             | yes      | The GitHub repository owner/organization.
| `Repository`        | yes      | The GitHub repository name.
| `Images`            | yes      | See [`Images`](#images).
| `LatestOnly`        | no       | If true, get only the tag from the latest github release.
| `VersionConstraint` | no       | A SemVer constraint used to filter the github releases.
| `VersionRegex`      | no       | If specified, only matching release tags will be considered. If a capture group is present, only its contents will be passed on.

##### `Images`

A list of images to be updated with the latest release tag. Each image will get the same tag as the GitHub release.

| Field | Required | Description |
| ----------------- | ------------- |------------- |
| `SourceImage`     | yes           | The GitHub repository name.
| `TargetImageName` | no            | The TargetImageName of the image in `config.yaml` that you want to update.

#### `HelmLatest`

The `HelmLatest` strategy templates out the latest version of configured
Helm charts and extracts image references from the rendered manifests. It
recursively searches for fields with an "image" key in the templated YAML
output.

| Field           | Required | Description |
| --------------- | ------------- |------------- |
| `HelmRepo`      | yes           | The URL of the Helm chart repository.
| `Charts`        | yes           | A map where keys are the charts to template, and values are another map from environment name to lists of helm values to `--set` in that environment. `helm template` is run once for each environment.
| `Images`        | no            | Used to map a given update image to an entry in `config.yaml`. There may be multiple entries that have the same `SourceImage`, but different `TargetImageName`s, so we need to choose which one receives the update image.
| `ImageDenylist` | no            | A list of images to exclude from the results.

#### `Registry`

The `Registry` strategy fetches all image tags that match the `VersionFilter` from a registry defined in the Images provided.
Supported registries are:
* Suse Container Registry (registry.suse.com)
* Docker Hub
* Quay.io
* K8s registry (registry.k8s.io)
* GitHub Container Registry (ghcr.io)
* Google Container Registry (gcr.io)

| Field           | Required | Description |
|-----------------|----------|------------- |
| `Images`        | yes      | Used to map a given update image to an entry in `config.yaml`. There may be multiple entries that have the same `SourceImage`, but different `TargetImageName`s, so we need to choose which one receives the update image.
| `Latest`        | no       | A flag to only use the latest tag. This only works if all tags are in semver format.
| `VersionFilter` | no       | A regex to match against the image tags fetched from the registry.
