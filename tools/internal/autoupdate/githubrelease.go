package autoupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v71/github"
	"github.com/rancher/image-mirror/internal/config"
)

// GithubRelease retrieves the tags of all github releases that match the VersionConstraint if
// LatestOnly is false, or the tag of the latest release if LatestOnly is true.
// It returns the configured Images with this tag.
// It assumes that the images have the same tags as the github releases.
type GithubRelease struct {
	Owner             string
	Repository        string
	Images            []AutoupdateImageRef
	LatestOnly        bool
	VersionConstraint string
}

func (gr *GithubRelease) GetUpdateImages() ([]*config.Image, error) {
	client := github.NewClient(nil)
	if gr.LatestOnly {
		return gr.getImagesFromLatestRelease(client)
	} else {
		return gr.getImagesFromAllReleases(client)
	}
}

func (gr *GithubRelease) getImagesFromAllReleases(client *github.Client) ([]*config.Image, error) {
	opt := &github.ListOptions{}

	images := make([]*config.Image, 0, len(gr.Images))
	for {
		releases, resp, err := client.Repositories.ListReleases(context.Background(), gr.Owner, gr.Repository, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to get releases: %w", err)
		}

		for _, release := range releases {
			if release.GetPrerelease() || release.GetDraft() {
				continue
			}
			if gr.VersionConstraint != "" {
				version, err := semver.NewVersion(release.GetTagName())
				if err != nil {
					return nil, fmt.Errorf("error parsing release version: %w", err)
				}

				constraint, err := semver.NewConstraint(gr.VersionConstraint)
				if err != nil {
					return nil, fmt.Errorf("error parsing version constraint: %w", err)
				}

				//check if tag matches the constraint
				if !constraint.Check(version) {
					continue
				}
			}
			for _, sourceImage := range gr.Images {
				image, err := config.NewImage(sourceImage.SourceImage, []string{release.GetTagName()})
				if err != nil {
					return nil, fmt.Errorf("failed to construct image from source image %q and tag %q: %w", sourceImage, release.GetTagName(), err)
				}
				image.SetTargetImageName(sourceImage.TargetImageName)
				images = append(images, image)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return images, nil
}

func (gr *GithubRelease) getImagesFromLatestRelease(client *github.Client) ([]*config.Image, error) {
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), gr.Owner, gr.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}
	latestTag := *release.TagName

	images := make([]*config.Image, 0, len(gr.Images))
	for _, sourceImage := range gr.Images {
		image, err := config.NewImage(sourceImage.SourceImage, []string{latestTag})
		if err != nil {
			return nil, fmt.Errorf("failed to construct image from source image %q and tag %q: %w", sourceImage, latestTag, err)
		}
		image.SetTargetImageName(sourceImage.TargetImageName)
		images = append(images, image)
	}

	return images, nil
}

func (gr *GithubRelease) Validate() error {
	if gr.Owner == "" {
		return errors.New("must specify Owner")
	}
	if gr.Repository == "" {
		return errors.New("must specify Repository")
	}
	if gr.Images == nil {
		return errors.New("must specify Images")
	} else if len(gr.Images) == 0 {
		return errors.New("must specify at least one element for Images")
	}
	if gr.LatestOnly && gr.VersionConstraint != "" {
		return errors.New("must not specify VersionConstraint when LatestOnly=true")
	}
	if gr.VersionConstraint != "" {
		_, err := semver.NewConstraint(gr.VersionConstraint)
		if err != nil {
			return fmt.Errorf("invalid VersionConstraint: %w", err)
		}
	}
	return nil
}
