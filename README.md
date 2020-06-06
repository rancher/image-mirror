# Mirroring external Images into Rancher Repo in Dockerhub

This repo is dedicated to mirror images from other organizations into Rancher. There are no packaging changes or changes in the layers of these images. 

## Mirroring Types

There are 2 types of images that are mirrored. 

* Single Arch Images - This list is maintained in the `images-list` file. The file is structured as `Name of Original Image`, `Name of Rancher Image`, `Image Tag`. 

* Multi-Arch Images - Coming Soon!

## New Images

When adding new images to the repo, please comment on the repo that this ia new image being mirrored.

An EIO team member or manager will need to create the repo in DockerHub as well as add the `automatedcipublisher` as a team member in DockerHub with `write` access in order for the images to be automatically pushed.

## Existing Images

Update the tag in the `images-list` repo for an updated image to be pulled/pushed.
