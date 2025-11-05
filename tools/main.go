package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rancher/image-mirror/internal/autoupdate"
	"github.com/rancher/image-mirror/internal/config"
	"github.com/rancher/image-mirror/internal/git"
	"github.com/rancher/image-mirror/internal/paths"
	"github.com/rancher/image-mirror/internal/regsync"

	"github.com/google/go-github/v77/github"
	"github.com/urfave/cli/v3"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
)

var dryRun bool
var entryName string
var mergeBaseBranch string

func main() {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-path",
				Aliases:     []string{"c"},
				Value:       "config.yaml",
				Usage:       "Path to config.yaml file",
				Destination: &paths.ConfigYaml,
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "autoupdate",
				Usage:  fmt.Sprintf("Use contents of %s to make pull requests that update %s", paths.AutoUpdateYaml, paths.ConfigYaml),
				Action: autoUpdate,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "dry-run",
						Aliases:     []string{"n"},
						Usage:       "Only print what would be done",
						Destination: &dryRun,
					},
					&cli.StringFlag{
						Name:        "entry",
						Aliases:     []string{"e"},
						Usage:       "Autoupdate specific entry instead of all",
						Destination: &entryName,
					},
				},
			},
			{
				Name:   "format",
				Usage:  "Enforce formatting on certain files",
				Action: formatFiles,
			},
			{
				Name:   "generate-regsync",
				Usage:  "Generate regsync.yaml",
				Action: generateRegsyncYaml,
			},
			{
				Name:   "validate",
				Usage:  "Validate the state of various files",
				Action: validate,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "merge-base-branch",
						Value:       "master",
						Usage:       "The branch to compare HEAD to to get the merge base",
						Destination: &mergeBaseBranch,
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

// generateRegsyncYaml regenerates the regsync config file from the current state
// of config.yaml.
func generateRegsyncYaml(_ context.Context, _ *cli.Command) error {
	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}

	regsyncYaml, err := configYaml.ToRegsyncConfig()
	if err != nil {
		return err
	}

	if err := regsync.WriteConfig(paths.RegsyncYaml, regsyncYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.RegsyncYaml, err)
	}

	return nil
}

func formatFiles(_ context.Context, _ *cli.Command) error {
	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}
	if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
	}

	autoUpdateYaml, err := autoupdate.Parse(paths.AutoUpdateYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.AutoUpdateYaml, err)
	}
	if err := autoupdate.Write(paths.AutoUpdateYaml, autoUpdateYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", paths.AutoUpdateYaml, err)
	}

	return nil
}

// autoUpdate uses the contents of autoupdate.yaml to make pull requests
// that update config.yaml.
func autoUpdate(ctx context.Context, _ *cli.Command) error {
	if !dryRun {
		if clean, err := git.IsWorkingTreeClean(); err != nil {
			return fmt.Errorf("failed to get status of working tree: %w", err)
		} else if !clean {
			return errors.New("working tree or index has changes")
		}
	}

	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}

	autoUpdateEntries, err := autoupdate.Parse(paths.AutoUpdateYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.AutoUpdateYaml, err)
	}

	ghClient := github.NewClient(nil)
	if !dryRun {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			return errors.New("must define GITHUB_TOKEN")
		}
		ghClient = ghClient.WithAuthToken(githubToken)
	}

	value := os.Getenv("GITHUB_REPOSITORY")
	if value == "" {
		return errors.New("must define GITHUB_REPOSITORY")
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return errors.New("must define GITHUB_REPOSITORY in form <owner>/<repo>")
	}
	githubOwner := parts[0]
	githubRepo := parts[1]

	errorPresent := false
	for _, autoUpdateEntry := range autoUpdateEntries {
		if entryName != "" && autoUpdateEntry.Name != entryName {
			fmt.Printf("%s: skipped\n", autoUpdateEntry.Name)
			continue
		}

		autoUpdateOptions := autoupdate.AutoUpdateOptions{
			BaseBranch:   "master",
			ConfigYaml:   configYaml.DeepCopy(),
			DryRun:       dryRun,
			GithubOwner:  githubOwner,
			GithubRepo:   githubRepo,
			GithubClient: ghClient,
		}
		if err := autoUpdateEntry.Run(ctx, autoUpdateOptions); err != nil {
			fmt.Printf("%s: error: %s\n", autoUpdateEntry.Name, err)
			errorPresent = true
			continue
		}
	}
	if errorPresent {
		return fmt.Errorf("one or more %s entries failed to update; please see above logs for details", paths.AutoUpdateYaml)
	}

	return nil
}

// validate is used to run validations based in Go code against
// the state of the image-mirror repo.
func validate(_ context.Context, _ *cli.Command) error {
	configYaml, err := config.Parse(paths.ConfigYaml)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", paths.ConfigYaml, err)
	}

	// Run validations
	errs := make([]error, 0)
	validateSourceImageAndTargetImageName(&errs, configYaml)
	validateNoTagsRemoved(&errs, configYaml)
	validateNewTagsPullable(&errs, configYaml)
	validateDockerHubRepoExists(&errs, configYaml)

	// Format results into one error, if any
	if len(errs) > 0 {
		outputErrs := make([]error, 0, len(errs)+1)
		outputErrs = append(outputErrs, errors.New("validation failed"))
		outputErrs = append(outputErrs, errs...)
		return errors.Join(outputErrs...)
	}

	return nil
}

func validateSourceImageAndTargetImageName(errs *[]error, configYaml *config.Config) {
	imageMap := map[config.ImageIndex]bool{}
	for _, image := range configYaml.Images {
		index := config.ImageIndex{
			SourceImage:     image.SourceImage,
			TargetImageName: image.TargetImageName(),
		}
		_, alreadyPresent := imageMap[index]
		if alreadyPresent {
			err := fmt.Errorf("found multiple images in %s with SourceImage %s and TargetImageName %s",
				paths.ConfigYaml, image.SourceImage, image.TargetImageName())
			*errs = append(*errs, err)
		} else {
			imageMap[index] = true
		}
	}
}

func validateNoTagsRemoved(errs *[]error, newConfigYaml *config.Config) {
	oldConfigYaml, err := loadMergeBaseConfigYaml(mergeBaseBranch)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to load %s from merge base %q: %w", paths.ConfigYaml, mergeBaseBranch, err))
		return
	}
	checkNoTagsRemoved(errs, oldConfigYaml.Images, newConfigYaml.Images)
}

func loadMergeBaseConfigYaml(branch string) (*config.Config, error) {
	mergeBase, err := git.GetMergeBase(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge base: %w", err)
	}
	oldContent, err := git.GetFileContentAtCommit(mergeBase, paths.ConfigYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content at %s: %w", mergeBase, err)
	}
	oldConfigYaml, err := config.ParseFromBytes(oldContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old %s: %w", paths.ConfigYaml, err)
	}
	return oldConfigYaml, nil
}

func checkNoTagsRemoved(errs *[]error, oldImages, newImages []*config.Image) {
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(newImages...)
	for _, oldImage := range oldImages {
		diffImage, err := accumulator.TagDifference(oldImage)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to diff image %s (TargetImageName %q): %w", oldImage.SourceImage, oldImage.TargetImageName(), err)
			*errs = append(*errs, wrappedErr)
			continue
		}
		if diffImage == nil {
			continue
		}
		for _, missedTag := range diffImage.Tags {
			err := fmt.Errorf("%s:%s removed (TargetImageName %q)", diffImage.SourceImage, missedTag, diffImage.TargetImageName())
			*errs = append(*errs, err)
		}
	}
}

func validateNewTagsPullable(errs *[]error, newConfigYaml *config.Config) {
	oldConfigYaml, err := loadMergeBaseConfigYaml(mergeBaseBranch)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to load %s from merge base %q: %w", paths.ConfigYaml, mergeBaseBranch, err))
		return
	}

	// Find the new tags
	imagesWithNewTags := make([]*config.Image, 0)
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(oldConfigYaml.Images...)
	for _, newImage := range newConfigYaml.Images {
		diffImage, err := accumulator.TagDifference(newImage)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to diff image %s (TargetImageName %q): %w", newImage.SourceImage, newImage.TargetImageName(), err)
			*errs = append(*errs, wrappedErr)
			continue
		}
		if diffImage == nil {
			continue
		}
		imagesWithNewTags = append(imagesWithNewTags, diffImage)
	}

	// Instantiate oras store
	dirPath, err := os.MkdirTemp("", "image-mirror-validation-*")
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to create temp dir: %w", err))
		return
	}
	defer os.RemoveAll(dirPath)
	store, err := oci.New(dirPath)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to instantiate oras store: %w", err))
		return
	}

	// Try pulling each tag
	for _, newTagImage := range imagesWithNewTags {
		repo, err := parseRepository(newTagImage.SourceImage)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to parse %s as repository: %w", newTagImage.SourceImage, err)
			*errs = append(*errs, wrappedErr)
			continue
		}
		// Workflows that are triggered by pull requests from forks (i.e.
		// every human-created PR in this repo) cannot get the ID token that
		// is needed to get secrets from EIO's setup. Pulling images from
		// the application collection requires one of these secrets. So,
		// we do not try pulling the image if it is from the appco.
		if strings.HasPrefix(newTagImage.SourceImage, "dp.apps.rancher.io") {
			continue
		}
		for _, newTag := range newTagImage.Tags {
			_, err := oras.Copy(context.Background(), repo, newTag, store, newTag, oras.DefaultCopyOptions)
			if err != nil {
				*errs = append(*errs, fmt.Errorf("failed to pull %s:%s: %w", newTagImage.SourceImage, newTag, err))
				continue
			}
		}
	}
}

func parseRepository(repository string) (*remote.Repository, error) {
	preparedRepository := repository
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid format")
	}
	if !strings.Contains(parts[0], ".") {
		preparedRepository = "docker.io/" + preparedRepository
	}
	repo, err := remote.NewRepository(preparedRepository)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate repository: %w", err)
	}
	return repo, nil
}

func validateDockerHubRepoExists(errs *[]error, newConfigYaml *config.Config) {
	oldConfigYaml, err := loadMergeBaseConfigYaml(mergeBaseBranch)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to load %s from merge base %q: %w", paths.ConfigYaml, mergeBaseBranch, err))
		return
	}

	// get images that were added in this branch
	newImages := make([]*config.Image, 0, len(newConfigYaml.Images))
	accumulator := config.NewImageAccumulator()
	accumulator.AddImages(oldConfigYaml.Images...)
	for _, newImage := range newConfigYaml.Images {
		if accumulator.Contains(newImage) {
			continue
		}
		if len(newImage.TargetRepositories) > 0 && !slices.Contains(newImage.TargetRepositories, "docker.io/rancher") {
			continue
		}
		newImages = append(newImages, newImage)
	}
	if len(newImages) == 0 {
		return
	}

	// fetch existing repositories from dockerhub
	existingRepositories, err := fetchDockerHubRepositories()
	if err != nil {
		*errs = append(*errs, fmt.Errorf("failed to fetch existing repositories from dockerhub: %w", err))
		return
	}

	for _, newImage := range newImages {
		targetImageName := newImage.TargetImageName()
		_, repoExists := existingRepositories[targetImageName]
		if !repoExists {
			*errs = append(*errs, fmt.Errorf("repository rancher/%s does not exist on dockerhub", targetImageName))
		}
	}
}

func fetchDockerHubRepositories() (map[string]struct{}, error) {
	type DockerAPIResponseRepository struct {
		Name string `json:"name"`
	}
	type DockerAPIResponse struct {
		Next    string                        `json:"next"`
		Results []DockerAPIResponseRepository `json:"results"`
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	repos := map[string]struct{}{}
	nextURL := "https://hub.docker.com/v2/namespaces/rancher/repositories?page_size=100"
	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
		}

		var apiResponse DockerAPIResponse
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&apiResponse); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		for _, repo := range apiResponse.Results {
			repos[repo.Name] = struct{}{}
		}

		// The URL for the next iteration is the 'next' field from the current response.
		// If 'next' is an empty string or null, the loop will terminate.
		nextURL = apiResponse.Next
	}

	return repos, nil
}
