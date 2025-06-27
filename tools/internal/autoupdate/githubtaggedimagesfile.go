package autoupdate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v71/github"
)

// GithubTaggedImagesFile finds images by iterating over every github release
// that matches a version constraint, and reading a text file from the commit that
// the tag on the release points to. Each line of the file should contain the image
// in "repository:tag" format.
type GithubTaggedImagesFile struct {
	Owner             string
	Repository        string
	ImagesFilePath    string
	VersionConstraint string
	Images            []AutoupdateImageRef `json:",omitempty"`
}

func (gtif *GithubTaggedImagesFile) GetUpdateImages() ([]*config.Image, error) {
	client := github.NewClient(nil)

	constraint, err := semver.NewConstraint(gtif.VersionConstraint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version constraint %q: %w", gtif.VersionConstraint, err)
	}

	// Get all releases
	var allReleases []*github.RepositoryRelease
	opts := &github.ListOptions{PerPage: 100}
	for {
		releases, resp, err := client.Repositories.ListReleases(context.Background(), gtif.Owner, gtif.Repository, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases: %w", err)
		}

		allReleases = append(allReleases, releases...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	imageMap := make(map[string][]string)
	for _, release := range allReleases {
		tagName := release.GetTagName()
		if *release.Draft || *release.Prerelease || tagName == "" {
			continue
		}
		version, err := semver.NewVersion(tagName)
		if err != nil {
			continue
		}

		// Skip versions that don't match constraint
		if !constraint.Check(version) {
			continue
		}

		// Get file content for this release
		fileContent, _, _, err := client.Repositories.GetContents(context.Background(), gtif.Owner, gtif.Repository, gtif.ImagesFilePath, &github.RepositoryContentGetOptions{
			Ref: tagName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get contents of %s for release %s", gtif.ImagesFilePath, tagName)
		}
		content, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("failed to decode file content for release %s: %w", tagName, err)
		}

		// Parse images from file content
		if err := gtif.parseImagesFromContent(content, imageMap); err != nil {
			return nil, fmt.Errorf("failed to parse images from file content for release %s: %w", tagName, err)
		}
	}

	// Convert imageMap to config.Image objects
	images := make([]*config.Image, 0, len(imageMap))
	for sourceImage, tags := range imageMap {
		targetImageName := ""
		for _, autoupdateImageRef := range gtif.Images {
			if sourceImage == autoupdateImageRef.SourceImage {
				targetImageName = autoupdateImageRef.TargetImageName
				break
			}
		}
		if targetImageName == "" {
			return nil, fmt.Errorf("found image %s but it is not present in Images", sourceImage)
		}
		image, err := config.NewImage(sourceImage, tags, targetImageName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for %s: %w", sourceImage, err)
		}
		images = append(images, image)
	}

	return images, nil
}

// parseImagesFromContent expects each line of content to contain an
// image in "repository:tag" format. For example:
//
// test-reg/test-repo1:v1.2.3
// test-reg/test-repo2:v4.5.6
// test-reg/test-repo3:v7.8.9
func (gtif *GithubTaggedImagesFile) parseImagesFromContent(content string, imageMap map[string][]string) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse image reference
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			return fmt.Errorf(`image %q did not split into two on ":"`, line)
		}
		repository := strings.TrimPrefix(parts[0], "docker.io/")
		tag := parts[1]
		if strings.HasPrefix("rancher/", repository) {
			continue
		}

		// Add tag to image map
		existingTags, present := imageMap[repository]
		if !present {
			imageMap[repository] = []string{tag}
		} else {
			if !slices.Contains(existingTags, tag) {
				imageMap[repository] = append(imageMap[repository], tag)
			}
		}
	}

	return scanner.Err()
}

func (gtif *GithubTaggedImagesFile) Validate() error {
	if gtif.Owner == "" {
		return errors.New("must specify Owner")
	}
	if gtif.Repository == "" {
		return errors.New("must specify Repository")
	}
	if gtif.ImagesFilePath == "" {
		return errors.New("must specify ImagesFilePath")
	}
	if gtif.VersionConstraint != "" {
		if _, err := semver.NewConstraint(gtif.VersionConstraint); err != nil {
			return fmt.Errorf("invalid VersionConstraint %q: %w", gtif.VersionConstraint, err)
		}
	}

	return nil
}
