#### Pull Request Checklist ####

- [ ] Change does not remove any existing Images or Tags in the images-list file
- [ ] Change does not remove / overwrite exiting Images or Tags in Rancher DockerHub
- [ ] New entries are in format `SOURCE DESTINATION TAG`
- [ ] New entries are added to the correct section of the list (sorted lexicographically)
- [ ] New entries have a repo created in Rancher Dockerhub (where the image will be mirrored to)
- [ ] Changes to scripting or CI config have been tested to the best of your ability

#### Types of Change ####

<!-- New image, version bump. script update, etc etc -->

#### Linked Issues ####

<!-- Link any related issues, pull-requests, or commit hashes that are relevant to this pull request.  -->

#### Additional Notes ####

<!-- Any additional details / test results / etc -->

#### Final Checks after the PR is merged ####
- [ ] Confirm that you can pull the new images and tags from DockerHub