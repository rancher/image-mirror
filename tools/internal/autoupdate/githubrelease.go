package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v77/github"
	"github.com/rancher/image-mirror/internal/config"
)

// GithubRelease retrieves the tags of all github releases that match the VersionConstraint if
// LatestOnly is false, or the tag of the latest release if LatestOnly is true.
// It returns the configured Images with this tag.
// It assumes that the images have the same tags as the github releases.
type GithubRelease struct {
	Owner                     string
	Repository                string
	Images                    []AutoupdateImageRef
	LatestOnly                bool                `json:",omitempty"`
	VersionConstraint         string              `json:",omitempty"`
	compiledVersionConstraint *semver.Constraints `json:"-"`
	// Only tags matching VersionRegex will be considered. If VersionRegex
	// contains a match group, the contents of the match group will be
	// used as the version.
	VersionRegex         string         `json:",omitempty"`
	compiledVersionRegex *regexp.Regexp `json:"-"`
}

func (gr *GithubRelease) GetUpdateImages() ([]*config.Image, error) {
	client := github.NewClient(nil)

	var tags []string
	if gr.LatestOnly {
		latestTag, err := gr.getVersionFromLatestRelease(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get tag from latest github release: %w", err)
		}
		tags = append(tags, latestTag)
	} else {
		ghTags, err := gr.getVersionsFromAllReleases(client)
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

func (gr *GithubRelease) getVersionsFromAllReleases(client *github.Client) ([]string, error) {
	opt := &github.ListOptions{}
	var versions []string
	for {
		releases, resp, err := client.Repositories.ListReleases(context.Background(), gr.Owner, gr.Repository, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to get releases: %w", err)
		}

		for _, release := range releases {
			if release.GetPrerelease() || release.GetDraft() {
				continue
			}
			tag := release.GetTagName()
			version, err := gr.processTagToVersion(tag)
			if err != nil {
				return nil, fmt.Errorf("failed to process tag into version: %w", err)
			}
			if version == "" {
				continue
			}
			versions = append(versions, version)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return versions, nil
}

func (gr *GithubRelease) getVersionFromLatestRelease(client *github.Client) (string, error) {
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), gr.Owner, gr.Repository)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}
	tag := release.GetTagName()
	version, err := gr.processTagToVersion(tag)
	if err != nil {
		return "", fmt.Errorf("failed to process tag into version: %w", err)
	}
	return version, nil
}

func (gr *GithubRelease) processTagToVersion(tag string) (string, error) {
	version := tag
	if gr.VersionRegex != "" {
		matches := gr.compiledVersionRegex.FindStringSubmatch(tag)
		switch len(matches) {
		case 0:
			return "", nil
		case 1:
			version = matches[0]
		default:
			version = matches[1]
		}
	}
	if gr.VersionConstraint != "" {
		version, err := semver.NewVersion(version)
		if err != nil {
			return "", fmt.Errorf("error parsing release version: %w", err)
		}
		if !gr.compiledVersionConstraint.Check(version) {
			return "", nil
		}
	}
	return version, nil
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
		compiledVersionConstraint, err := semver.NewConstraint(gr.VersionConstraint)
		if err != nil {
			return fmt.Errorf("invalid VersionConstraint: %w", err)
		}
		gr.compiledVersionConstraint = compiledVersionConstraint
	}
	if gr.VersionRegex != "" {
		compiledVersionRegex, err := regexp.Compile(gr.VersionRegex)
		if err != nil {
			return fmt.Errorf("invalid VersionRegex: %w", err)
		}
		gr.compiledVersionRegex = compiledVersionRegex
	}
	return nil
}
