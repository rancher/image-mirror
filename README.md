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
](https://github.com/rancher/image-mirror/actions/workflows/add-tag-to-existing-image.yml). The configuration lives in [`config.json`](https://github.com/rancher/image-mirror/blob/master/retrieve-image-tags/config.json). The basic structure is having a descriptive key (pick your own), and specify the list of images for which the available tag(s) need to be looked up (`versionSource`), and an optional SemVer constraint if you need to limit what tags are used. The current datasources are:

- `github-releases`: This will use GitHub releases as source, excluding pre-releases. This can be used if you need to keep all tags from the configured images in sync with GitHub releases
- `github-latest-release`: This will use the release on GitHub marked as `Latest release`. This can be used if you only want one release to be added that is marked as latest.
- `github-tagged-images-file`: This will look up GitHub git repository tags, and find the list of images inside a specified file. The tag must have an associated release, with the pre-release flag unset. This can be used if your project maintains a list of images in a file, e.g., https://github.com/longhorn/longhorn/blob/master/deploy/longhorn-images.txt
- `registry`: This will use the registry of the first image and look up available tags.
- `helm-latest:helm-repo-fqdn`: This will add the helm-repo-fqdn, and use the latest version of configured Helm chart(s) (`helmCharts`) configured to extract the images. It uses `helm template` and `helm show values` to extract images. You can specify one ore more iterations of `helm template` by specifying one ore more `values` configurations to make sure all required images are extracted. If you want to block certain images from being extracted, you can use `imageDenylist` in the configuration. See example below.
- `helm-oci`: This is the same as `helm-latest`, except you don't need to provide a repository but it will use the charts directly from the provided `helmCharts` (which should be formatted as `oci://hostname/chart`).
- `helm-directory:/full_path_to_charts_directory`: Provide a directory with chart(s) to use, introduced for testing purposes. The full path used by the `helm` commands is `/full_path_to_charts_directory/chart_name_from_config`.

The current filters for tags are:

- `versionConstraint`: This is a semver constraint that will match the given expression and ignore tags that do not match.
- `versionFilter`: This is a regex filter that will match the given expression and ignore tags that do not match.
- `latest`: Sorts the found tags numerically and returns only the latest tag
- `latest_entry`: Returns the last found (newest) tag only (can be used when tags are not semver/cannot be sorted numerically)

`github-tagged-images-file` specific options:
- `imagesFilePath`: the path to the list of images inside a GitHub git repository

Helm specific options:

- `imageDenylist`: An array of images that will not be added (in case the image matching finds images that shouldn't be added as the automation only accounts for adding tags to existing images, not adding new images as they need to be approved first)
- `kubeVersion`: What version to pass to `--kube-version` when running `helm template`
- `devel`: Use chart development versions (adds `--devel` to `helm template` and `helm show values` commands)
- `additionalVersionFilter`: Next to retrieving the latest Helm chart, it will also run `helm template` and `helm show values` commands with `--version` parameters from this array. This is useful if you want to include images from multiple versions in a single pull request.
- `versionFilter`: Specify what version of the Helm chart needs to be used (this will only run `helm template` and `helm show values` with the configured `versionFilter`)

See example configuration for `github-releases`, `github-latest-release` and `registry`:

```json
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

See example configuration for `github-tagged-images-file`:
```json
{
  "longhorn": {
    "versionSource": "github-tagged-images-file:longhorn/longhorn",
    "imagesFilePath": "deploy/longhorn-images.txt",
    "versionConstraint": ">=1.4.0"
  }
}
```

See example configuration for `helm-latest:helm-repo-fqdn`:

```json
{
  "cilium": {
    "versionSource": "helm-latest:https://helm.cilium.io",
    "imageDenylist": [
      "quay.io/cilium/operator",
      "quay.io/cilium/startup-script"
    ],
    "helmCharts": {
      "cilium": {
        "devel": true,
        "chartConfig": {
          "aws": {
            "values": [
              "eni.enabled=true"
            ],
            "kubeVersion": "1.24"
          },
          "azure":  {
            "values": [
              "azure.enabled=true"
            ]
          },
          "generic": {
            "values": [
              "clustermesh.useAPIServer=true",
              "envoy.enabled=true",
              "hubble.ui.enabled=true",
              "hubble.relay.enabled=true",
              "hubble.enabled=true"
            ]
          },
          "kubeversiononly": {
            "kubeVersion": "1.28"
          }
        }
      }
    }
  },
  "epinio": {
    "versionSource": "helm-latest:https://epinio.github.io/helm-charts",
    "helmCharts": {
      "epinio": {
        "chartConfig": {
          "generic": {
            "values": [
              "global.domain=myepiniodomain.org"
            ]
          }
        }
      }
    }
  },
  "kubewarden": {
    "versionSource": "helm-latest:https://charts.kubewarden.io",
    "helmCharts": {
      "kubewarden-controller": {},
      "kubewarden-defaults": {}
    }
  },
  "neuvector": {                                                                                                                                                                                                                                                                                                             
    "versionSource": "helm-latest:https://neuvector.github.io/neuvector-helm",
    "helmCharts": {
      "core": {}
    }
  },
  "longhorn": {
    "versionSource": "helm-latest:https://charts.longhorn.io",
    "additionalVersionFilter": [
      "v1.4.*"
    ],
    "helmCharts": {
      "longhorn": {}
    }
  },
  "longhorn": {
    "versionSource": "helm-latest:https://charts.longhorn.io",
    "helmCharts": {
      "longhorn": {
        "versionFilter": "v1.4.*"
      }
    }
  }

}
```

See example configuration for `helm-oci`:

```
{
  "elemental": {
    "versionSource": "helm-oci",
    "imageDenylist": [
      "registry.suse.com/rancher/elemental-teal-channel"
    ],
    "helmCharts": {
      "oci://registry.suse.com/rancher/elemental-operator-chart": {}
    }
  }
}
```

See example configuration for `helm-directory`:

```
{
  "epinio-directory": {
    "versionSource": "helm-directory:/epinio-charts/chart",
    "helmCharts": {
      "epinio": {
        "chartConfig": {
          "generic": {
            "values": [
              "global.domain=myepiniodomain.org"
            ]
          }
        }
      }
    }
  }
}
```

If you want to manually test your configuration changes to check if the correct tags are found, you can use the following commands depending on your available runtime:

#### Docker 

```
docker run -v $PWD:/code -w /code/retrieve-image-tags python:3.10-alpine sh -c "apk -qU add helm && pip install --disable-pip-version-check --root-user-action=ignore -qr requirements.txt && python retrieve-image-tags.py"
```

#### podman

```
podman run -v $PWD:/code -w /code/retrieve-image-tags python:3.10-alpine sh -c "apk -qU add helm && pip install --disable-pip-version-check --root-user-action=ignore -qr requirements.txt && python retrieve-image-tags.py"
```

#### containerd

```
ctr images pull docker.io/library/python:3.10-alpine
ctr run -t --net-host --mount type=bind,src=$PWD,dst=/code,options=rbind:ro --cwd /code/retrieve-image-tags --rm docker.io/library/python:3.10-alpine workflow-test sh -c "apk -qU add helm && pip install --disable-pip-version-check --root-user-action=ignore -qr requirements.txt && python retrieve-image-tags.py"
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

There is also a wrapper script to support supplying images with tags. This was added to support the `helm-latest` version source which extracts images from Helm charts and does not work with the images + tags inputs. The wrapper script for full images can be used as follows:

```
FULL_IMAGES=quay.io/skopeo/stable:v1.13.3,quay.io/cilium/cilium-envoy:v1.25.9-e198a2824d309024cb91fb6a984445e73033291d make add-full-image-wrapper.sh
```

The wrapper script will run the `add-tag-to-existing-image.sh` script for each image, to be aligned with all the checks that are required.

Optionally, you can also check if the newly added image tag exists (this will also be run in GitHub Action):

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

or

```
Full Images: quay.io/skopeo/stable:v1.13.3,quay.io/cilium/cilium-envoy:v1.25.9-e198a2824d309024cb91fb6a984445e73033291d
```
