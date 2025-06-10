package autoupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/google/go-github/v71/github"
)

// GithubLatestRelease retrieves the tag of the latest release
// of the configured github repository, and returns the configured
// Images with this tag. It assumes that the images have the same
// tags as the github releases.
type GithubLatestRelease struct {
	Owner      string
	Repository string
	Images     []string
}

func (glr *GithubLatestRelease) GetUpdateImages() ([]*config.Image, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), glr.Owner, glr.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}
	latestTag := *release.TagName

	images := make([]*config.Image, 0, len(glr.Images))
	for _, sourceImage := range glr.Images {
		image, err := config.NewImage(sourceImage, []string{latestTag})
		if err != nil {
			return nil, fmt.Errorf("failed to construct image from source image %q and tag %q: %w", sourceImage, latestTag, err)
		}
		images = append(images, image)
	}

	return images, nil
}

func (glr *GithubLatestRelease) Validate() error {
	if glr.Owner == "" {
		return errors.New("must specify Owner")
	}
	if glr.Repository == "" {
		return errors.New("must specify Repository")
	}
	if glr.Images == nil {
		return errors.New("must specify Images")
	} else if len(glr.Images) == 0 {
		return errors.New("must specify at least one element for Images")
	}
	return nil
}
