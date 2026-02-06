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

	"github.com/rancher/artifact-mirror/internal/config"
	"github.com/rancher/artifact-mirror/internal/git"
	"github.com/rancher/artifact-mirror/internal/paths"
	"github.com/rancher/artifact-mirror/internal/regsync"

	"github.com/google/go-github/v80/github"
	"sigs.k8s.io/yaml"
)

type ConfigEntry struct {
	Name          string
	GithubRelease *GithubRelease `json:",omitempty"`
	HelmLatest    *HelmLatest    `json:",omitempty"`
	Registry      *Registry      `json:",omitempty"`
	Reviewers     []string       `json:",omitempty"`
}

type AutoUpdateOptions struct {
	BaseBranch   string
	ConfigYaml   *config.Config
	DryRun       bool
	GithubOwner  string
	GithubRepo   string
	GithubClient *github.Client
}

// AutoupdateArtifactRef is used to map a given update artifact to an entry in config.yaml.
// There may be multiple entries that have the same SourceArtifact, but different
// TargetArtifactName, so we need to choose which one receives the update artifact.
type AutoupdateArtifactRef struct {
	SourceArtifact     string
	TargetArtifactName string `json:",omitempty"`
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
	count := 0
	if entry.GithubRelease != nil {
		count++
	}
	if entry.HelmLatest != nil {
		count++
	}
	if entry.Registry != nil {
		count++
	}

	if count > 1 {
		return errors.New("must specify only one autoupdate strategy")
	}
	if count == 0 {
		return errors.New("must specify an autoupdate strategy")
	}

	if entry.GithubRelease != nil {
		if err := entry.GithubRelease.Validate(); err != nil {
			return fmt.Errorf("GithubRelease failed validation: %w", err)
		}
	} else if entry.HelmLatest != nil {
		if err := entry.HelmLatest.Validate(); err != nil {
			return fmt.Errorf("HelmLatest failed validation: %w", err)
		}
	} else if entry.Registry != nil {
		if err := entry.Registry.Validate(); err != nil {
			return fmt.Errorf("Registry failed validation: %w", err)
		}
	}

	if len(entry.Reviewers) == 0 {
		return errors.New("must specify at least one reviewer")
	}
	for _, reviewer := range entry.Reviewers {
		parts := strings.Split(reviewer, "/")
		if len(parts) > 2 {
			return fmt.Errorf("invalid reviewer format for %q: must be a username or in 'org/team' format", reviewer)
		}
		if len(parts) == 2 && (parts[0] == "" || parts[1] == "") {
			return fmt.Errorf("invalid reviewer format for %q: org and team must not be empty", reviewer)
		}
	}

	return nil
}

// GetUpdateArtifacts returns a slice of Artifacts that depends on the
// configured update strategy. The returned Artifacts may be from
// any source, and they may be gathered in any way. The intention
// is that they are new Artifacts (or new tags of existing Artifacts) that
// we want to mirror.
func (entry ConfigEntry) GetUpdateArtifacts() ([]*config.Artifact, error) {
	switch {
	case entry.GithubRelease != nil:
		return entry.GithubRelease.GetUpdateArtifacts()
	case entry.HelmLatest != nil:
		return entry.HelmLatest.GetUpdateArtifacts()
	case entry.Registry != nil:
		return entry.Registry.GetUpdateArtifacts()
	default:
		return nil, errors.New("did not find update strategy")
	}
}

func (entry ConfigEntry) Run(ctx context.Context, opts AutoUpdateOptions) error {
	newArtifacts, err := entry.GetUpdateArtifacts()
	if err != nil {
		return fmt.Errorf("failed to get latest artifacts for %s: %w", entry.Name, err)
	}

	accumulator := config.NewArtifactAccumulator()
	accumulator.AddArtifacts(opts.ConfigYaml.Artifacts...)

	artifactsToUpdate := make([]*config.Artifact, 0, len(newArtifacts))
	for _, latestArtifact := range newArtifacts {
		artifactToUpdate, err := accumulator.TagDifference(latestArtifact)
		if err != nil {
			return fmt.Errorf("failed to get tag difference for artifact %s: %w", latestArtifact.SourceArtifact, err)
		}
		if artifactToUpdate != nil {
			artifactsToUpdate = append(artifactsToUpdate, artifactToUpdate)
		}
	}
	if len(artifactsToUpdate) == 0 {
		fmt.Printf("%s: no updates found\n", entry.Name)
		return nil
	}

	artifactSetHash, err := hashArtifactSet(artifactsToUpdate)
	if err != nil {
		return fmt.Errorf("failed to hash set of artifacts that need updates: %w", err)
	}
	branchName := fmt.Sprintf("autoupdate/%s/%s", entry.Name, artifactSetHash)

	// When filtering pull requests by head branch, the github API
	// requires that the head branch is in the format <owner>:<branch>.
	// In the case of branches pushed using GITHUB_TOKEN in rancher/artifact-mirror,
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
		msg := fmt.Sprintf("%s: would make PR under branch %s that adds:\n", entry.Name, branchName)
		for _, artifactToUpdate := range artifactsToUpdate {
			for _, fullArtifact := range artifactToUpdate.CombineSourceArtifactAndTags() {
				msg = msg + "  - " + fullArtifact + "\n"
			}
		}
		fmt.Print(msg)
		return nil
	}

	return entry.CreateArtifactUpdatePullRequest(ctx, opts, branchName, artifactsToUpdate)
}

func (entry ConfigEntry) CreateArtifactUpdatePullRequest(ctx context.Context, opts AutoUpdateOptions, branchName string, artifactsToUpdate []*config.Artifact) error {
	accumulator := config.NewArtifactAccumulator()
	accumulator.AddArtifacts(opts.ConfigYaml.Artifacts...)

	if err := git.CreateAndCheckoutBranch(opts.BaseBranch, branchName); err != nil {
		return fmt.Errorf("failed to create and checkout branch %s: %w", branchName, err)
	}
	for _, artifactToUpdate := range artifactsToUpdate {
		// We can reuse the accumulator here because we are making a sequence
		// of commits, each of which makes an addition from artifactsToUpdate.
		configYaml := opts.ConfigYaml
		accumulator.AddArtifacts(artifactToUpdate)
		configYaml.Artifacts = accumulator.Artifacts()
		if err := config.Write(paths.ConfigYaml, configYaml); err != nil {
			return fmt.Errorf("failed to write %s: %w", paths.ConfigYaml, err)
		}

		regsyncYaml, err := configYaml.ToRegsyncConfig()
		if err != nil {
			return fmt.Errorf("failed to generate regsync config for commit for artifact %s: %w", artifactToUpdate.SourceArtifact, err)
		}
		if err := regsync.WriteConfig(paths.RegsyncYaml, regsyncYaml); err != nil {
			return fmt.Errorf("failed to write regsync config for commit for artifact %s: %w", artifactToUpdate.SourceArtifact, err)
		}

		tagString := strings.Join(artifactToUpdate.Tags, ", ")
		msg := fmt.Sprintf("Add tag(s) %s for artifact %s", tagString, artifactToUpdate.SourceArtifact)
		if err := git.Commit(msg); err != nil {
			return fmt.Errorf("failed to commit changes for artifact %s: %w", artifactToUpdate.SourceArtifact, err)
		}
	}
	if err := git.PushBranch(branchName, "origin"); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	tagCount := 0
	for _, artifactToUpdate := range artifactsToUpdate {
		tagCount = tagCount + len(artifactToUpdate.Tags)
	}
	title := fmt.Sprintf("[autoupdate] Add %d tag(s) for `%s`", tagCount, entry.Name)
	body := "This PR was created by the autoupdate workflow.\n\nIt adds the following artifact tags:"
	for _, artifactToUpdate := range artifactsToUpdate {
		for _, fullArtifact := range artifactToUpdate.CombineSourceArtifactAndTags() {
			body = body + "\n- `" + fullArtifact + "`"
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

// hashArtifactSet computes a human-readable hash from a passed
// set of Artifacts. Immune to different order of Artifacts, and
// immune to the order of the tags in those Artifacts.
func hashArtifactSet(artifacts []*config.Artifact) (string, error) {
	for _, artifact := range artifacts {
		artifact.Sort()
	}
	slices.SortStableFunc(artifacts, config.CompareArtifacts)

	hasher := sha256.New()
	for _, artifact := range artifacts {
		for _, fullArtifact := range artifact.CombineSourceArtifactAndTags() {
			_, err := io.WriteString(hasher, fullArtifact)
			if err != nil {
				return "", fmt.Errorf("failed to write full artifact %q: %w", fullArtifact, err)
			}
		}
	}
	output := hasher.Sum(nil)
	strHash := base32.StdEncoding.EncodeToString(output)
	lowerStrHash := strings.ToLower(strHash)
	return lowerStrHash[:8], nil
}
