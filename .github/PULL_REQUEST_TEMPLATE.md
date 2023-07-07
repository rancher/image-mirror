#### Pull Request Checklist ####

- [ ] Change does not remove any existing Images or Tags in the images-list file
- [ ] Change does not remove / overwrite exiting Images or Tags in Rancher DockerHub
- [ ] If updating an existing entry, verify the `SOURCE` is still accurate and upstream hasn't been migrated to a new regitry or repo (if they've migrated, a new repo request to EIO is needed to comply with the `SOURCE DESTINATION TAG` pattern)
- [ ] New entries are in format `SOURCE DESTINATION TAG`
- [ ] New entries are added to the correct section of the list (sorted lexicographically)
- [ ] New entries have a repo created in Rancher Dockerhub (where the image will be mirrored to)
- [ ] New entries are licensed with Rancher favored licenses - Apache 2 and MIT - or approved licenses - as according to [CNCF approved licenses](https://github.com/cncf/foundation/blob/main/allowed-third-party-license-policy.md).
- [ ] New entries, when used in Rancher or Rancher's provided charts, have their corresponding origin added in Rancher's images [origins](https://github.com/rancher/rancher/blob/release/v2.7/pkg/image/origins.go) file (must be added for all Rancher versions `>= v2.7`).
- [ ] Changes to scripting or CI config have been tested to the best of your ability

#### Types of Change ####

<!-- New image, version bump. script update, etc etc -->

#### Linked Issues ####

<!-- Link any related issues, pull-requests, or commit hashes that are relevant to this pull request.  -->

#### Additional Notes ####

<!-- Any additional details / test results / etc -->

#### Final Checks after the PR is merged ####
- [ ] Confirm that you can pull the new images and tags from DockerHub
