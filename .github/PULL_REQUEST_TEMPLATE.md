#### Pull Request Checklist ####

- [ ] If an entire image is being added, a repo has been created for the image in DockerHub under the `rancher` org
- [ ] Additions are licensed with Rancher favored licenses - Apache 2 and MIT - or approved licenses - as according to [CNCF approved licenses](https://github.com/cncf/foundation/blob/main/allowed-third-party-license-policy.md).
- [ ] Additions, when used in Rancher or Rancher's provided charts, have their corresponding origin added in Rancher's images [origins](https://github.com/rancher/rancher/blob/release/v2.7/pkg/image/origins.go) file (must be added for all Rancher versions `>= v2.7`).
- [ ] Any changes to scripting or CI config have been tested to the best of your ability

#### Change Description ####

<!-- Describe what you are doing and why. Link any related issues, pull requests and commit hashes. -->

#### Final Checks after the PR is merged ####

- [ ] Confirm that you can pull the mirrored images and tags from all target locations
