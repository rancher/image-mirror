package autoupdate

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rancher/image-mirror/internal/config"
	"github.com/rancher/image-mirror/internal/git"
	"github.com/rancher/image-mirror/internal/paths"
	"github.com/rancher/image-mirror/internal/regsync"

	"github.com/google/go-github/v71/github"
	"sigs.k8s.io/yaml"
)

type ConfigEntry struct {
	Name                string
	GithubLatestRelease *GithubLatestRelease `json:",omitempty"`
	HelmLatest          *HelmLatest          `json:",omitempty"`
}

type AutoUpdateOptions struct {
	BaseBranch   string
	ConfigYaml   *config.Config
	DryRun       bool
	GithubOwner  string
	GithubRepo   string
	GithubClient *github.Client
}

func Parse(filePath string) ([]ConfigEntry, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	config := make([]ConfigEntry, 0)
	if err := yaml.UnmarshalStrict(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	for _, entry := range config {
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("entry %q failed validation: %w", entry.Name, err)
		}
	}

	return config, nil
}

func Write(filePath string, config []ConfigEntry) error {
	slices.SortStableFunc(config, func(a, b ConfigEntry) int {
		return strings.Compare(a.Name, b.Name)
	})

	contents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	if err := os.WriteFile(filePath, contents, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (entry ConfigEntry) Validate() error {
	if entry.Name == "" {
		return errors.New("must specify Name")
	}

	if entry.GithubLatestRelease == nil && entry.HelmLatest == nil {
		return errors.New("must specify an autoupdate strategy")
	} else if entry.GithubLatestRelease != nil && entry.HelmLatest != nil {
		return errors.New("must specify only one autoupdate strategy")
	} else if entry.GithubLatestRelease != nil {
		if err := entry.GithubLatestRelease.Validate(); err != nil {
			return fmt.Errorf("GithubLatestRelease failed validation: %w", err)
		}
	} else if entry.HelmLatest != nil {
		if err := entry.HelmLatest.Validate(); err != nil {
			return fmt.Errorf("HelmLatest failed validation: %w", err)
		}
	}

	return nil
}

// GetUpdateImages returns a slice of Images that depends on the
// configured update strategy. The returned Images may be from
// any source, and they may be gathered in any way. The intention
// is that they are new Images (or new tags of existing Images) that
// we want to mirror.
func (entry ConfigEntry) GetUpdateImages() ([]*config.Image, error) {
	switch {
	case entry.GithubLatestRelease != nil:
		return entry.GithubLatestRelease.GetUpdateImages()
	case entry.HelmLatest != nil:
		return entry.HelmLatest.GetUpdateImages()
	default:
		return nil, errors.New("did not find update strategy")
	}
}

func (entry ConfigEntry) Run(ctx context.Context, opts AutoUpdateOptions) error {
	newImages, err := entry.GetUpdateImages()
	if err != nil {
		return fmt.Errorf("failed to get latest images for %s: %w", entry.Name, err)
	}

	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(opts.ConfigYaml.Images...)

	imagesToUpdate := make([]*config.Image, 0, len(newImages))
	for _, latestImage := range newImages {
		imageToUpdate, err := accumulator.TagDifference(latestImage)
		if err != nil {
			return fmt.Errorf("failed to get tag difference for image %s: %w", latestImage.SourceImage, err)
		}
		if imageToUpdate != nil {
			imagesToUpdate = append(imagesToUpdate, imageToUpdate)
		}
	}
	if len(imagesToUpdate) == 0 {
		fmt.Printf("%s: no updates found\n", entry.Name)
		return nil
	}

	imageSetHash, err := hashImageSet(imagesToUpdate)
	if err != nil {
		return fmt.Errorf("failed to hash set of images that need updates: %w", err)
	}
	branchName := fmt.Sprintf("autoupdate/%s/%s", entry.Name, imageSetHash)

	// When filtering pull requests by head branch, the github API
	// requires that the head branch is in the format <owner>:<branch>.
	// In the case of branches pushed using GITHUB_TOKEN in rancher/image-mirror,
	// owner is "rancher". When running in a personal repo, setting GITHUB_TOKEN
	// to a PAT makes owner the same as the user's github username.
	headBranch := opts.GithubOwner + ":" + branchName
	requestContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pullRequests, _, err := opts.GithubClient.PullRequests.List(requestContext, opts.GithubOwner, opts.GithubRepo, &github.PullRequestListOptions{
		Head:  headBranch,
		Base:  opts.BaseBranch,
		State: "all",
	})
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}
	if len(pullRequests) == 1 {
		fmt.Printf("%s: found existing PR with head branch %s: %s\n", entry.Name, headBranch, pullRequests[0].GetHTMLURL())
		return nil
	} else if len(pullRequests) > 1 {
		pullRequestString := ""
		for _, pullRequest := range pullRequests {
			pullRequestString = pullRequestString + "\n- " + pullRequest.GetHTMLURL()
		}
		fmt.Printf("%s: warning: found multiple existing PRs with head branch %s:%s\n", entry.Name, headBranch, pullRequestString)
		return nil
	}

	if opts.DryRun {
		fmt.Printf("%s: would make PR under branch %s\n", entry.Name, branchName)
		return nil
	}

	return entry.CreateImageUpdatePullRequest(ctx, opts, branchName, imagesToUpdate)
}

func (entry ConfigEntry) CreateImageUpdatePullRequest(ctx context.Context, opts AutoUpdateOptions, branchName string, imagesToUpdate []*config.Image) error {
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(opts.ConfigYaml.Images...)

	if err := git.CreateAndCheckoutBranch(opts.BaseBranch, branchName); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s: %w", branchName, err)
	}
	for _, imageToUpdate := range imagesToUpdate {
		// We can reuse the accumulator here because we are making a sequence
		// of commits, each of which makes an addition from imagesToUpdate.
		configYaml := opts.ConfigYaml
		accumulator.AddImages(imageToUpdate)
		configYaml.Images = accumulator.Images()
		if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
			return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
		}

		regsyncYaml, err := configYaml.ToRegsyncConfig()
		if err != nil {
			return fmt.Errorf("failed to generate regsync config for commit for image %s: %w", imageToUpdate.SourceImage, err)
		}
		if err := regsync.WriteConfig(paths.RegsyncYaml, regsyncYaml); err != nil {
			return fmt.Errorf("failed to write regsync config for commit for image %s: %w", imageToUpdate.SourceImage, err)
		}

		tagString := strings.Join(imageToUpdate.Tags, ", ")
		msg := fmt.Sprintf("Add tag(s) %s for image %s", tagString, imageToUpdate.SourceImage)
		if err := git.Commit(msg); err != nil {
			return fmt.Errorf("failed to commit changes for image %s: %w", imageToUpdate.SourceImage, err)
		}
	}
	if err := git.PushBranch(branchName, "origin"); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	tagCount := 0
	for _, imageToUpdate := range imagesToUpdate {
		tagCount = tagCount + len(imageToUpdate.Tags)
	}
	title := fmt.Sprintf("[autoupdate] Add %d tag(s) for `%s`", tagCount, entry.Name)
	body := "This PR was created by the autoupdate workflow.\n\nIt adds the following image tags:"
	for _, imageToUpdate := range imagesToUpdate {
		for _, fullImage := range imageToUpdate.CombineSourceImageAndTags() {
			body = body + "\n- `" + fullImage + "`"
		}
	}
	maintainerCanModify := true
	newPullRequest := &github.NewPullRequest{
		Base:                &opts.BaseBranch,
		Head:                &branchName,
		Title:               &title,
		Body:                &body,
		MaintainerCanModify: &maintainerCanModify,
	}

	requestContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pullRequest, _, err := opts.GithubClient.PullRequests.Create(requestContext, opts.GithubOwner, opts.GithubRepo, newPullRequest)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	fmt.Printf("%s: created pull request: %s\n", entry.Name, pullRequest.GetHTMLURL())

	return nil
}

// hashImageSet computes a human-readable hash from a passed
// set of Images. Immune to different order of Images, and
// immune to the order of of the tags in those Images.
func hashImageSet(images []*config.Image) (string, error) {
	for _, image := range images {
		image.Sort()
	}
	slices.SortStableFunc(images, config.CompareImages)

	hasher := sha256.New()
	for _, image := range images {
		for _, fullImage := range image.CombineSourceImageAndTags() {
			_, err := io.WriteString(hasher, fullImage)
			if err != nil {
				return "", fmt.Errorf("failed to write full image %q: %w", fullImage, err)
			}
		}
	}
	output := hasher.Sum(nil)
	strHash := base32.StdEncoding.EncodeToString(output)
	lowerStrHash := strings.ToLower(strHash)
	return lowerStrHash[:8], nil
}
