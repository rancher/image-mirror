package autoupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v76/github"
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
	LatestOnly        bool   `json:",omitempty"`
	VersionConstraint string `json:",omitempty"`
}

func (gr *GithubRelease) GetUpdateImages() ([]*config.Image, error) {
	client := github.NewClient(nil)

	var tags []string
	if gr.LatestOnly {
		latestTag, err := gr.getTagFromLatestRelease(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get tag from latest github release: %w", err)
		}
		tags = append(tags, latestTag)
	} else {
		ghTags, err := gr.getTagsFromAllReleases(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get tags from github releases: %w", err)
		}
		tags = append(tags, ghTags...)
	}

	images := make([]*config.Image, 0, len(gr.Images))
	for _, sourceImage := range gr.Images {
		image, err := config.NewImage(sourceImage.SourceImage, tags, sourceImage.TargetImageName, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to construct image from source image %q and tags %v: %w", sourceImage, tags, err)
		}
		images = append(images, image)
	}
	return images, nil
}

func (gr *GithubRelease) getTagsFromAllReleases(client *github.Client) ([]string, error) {
	var constraint *semver.Constraints
	if gr.VersionConstraint != "" {
		var err error
		constraint, err = semver.NewConstraint(gr.VersionConstraint)
		if err != nil {
			return nil, fmt.Errorf("error parsing version constraint: %w", err)
		}
	}

	opt := &github.ListOptions{}
	var tags []string
	for {
		releases, resp, err := client.Repositories.ListReleases(context.Background(), gr.Owner, gr.Repository, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to get releases: %w", err)
		}

		for _, release := range releases {
			if release.GetPrerelease() || release.GetDraft() {
				continue
			}
			if constraint != nil {
				version, err := semver.NewVersion(release.GetTagName())
				if err != nil {
					return nil, fmt.Errorf("error parsing release version: %w", err)
				}

				//check if tag matches the constraint
				if !constraint.Check(version) {
					continue
				}
			}
			tags = append(tags, release.GetTagName())
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return tags, nil
}

func (gr *GithubRelease) getTagFromLatestRelease(client *github.Client) (string, error) {
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), gr.Owner, gr.Repository)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}

	return release.GetTagName(), nil
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
