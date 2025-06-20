package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images       []*Image
	Repositories []Repository
}

type Repository struct {
	// BaseUrl is used exclusively for building the target image ref
	// for a given image for a repository. For example, a target
	// image name of "mirrored-rancher-cis-operator" and a BaseUrl
	// of "docker.io/rancher" produce a target image ref of
	// "docker.io/rancher/mirrored-rancher-cis-operator".
	BaseUrl string
	// Whether the repository should have images mirrored to it.
	Target bool
	// Password is what goes into the "pass" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Password string
	// Registry is what goes into the "registry" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Registry string
	// RepoAuth goes into the "repoAuth" field of regsync.yaml in this
	// repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	RepoAuth bool `json:",omitempty"`
	// ReqConcurrent is what goes into the "reqConcurrent" field of
	// regsync.yaml for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	ReqConcurrent int `json:",omitempty"`
	// Username is what goes into the "user" field of regsync.yaml
	// for this repository. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Username string
}

func Parse(fileName string) (*Config, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	for _, image := range config.Images {
		if err := image.setDefaults(); err != nil {
			return nil, fmt.Errorf("failed to set defaults for image %q: %w", image.SourceImage, err)
		}
	}

	return config, nil
}

func Write(fileName string, config *Config) error {
	config.Sort()

	contents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	if err := os.WriteFile(fileName, contents, 0o644); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	return nil
}

func (config *Config) Sort() {
	for _, image := range config.Images {
		image.Sort()
	}
	slices.SortStableFunc(config.Images, CompareImages)
	slices.SortStableFunc(config.Repositories, compareRepositories)
}

func (config *Config) ToRegsyncConfig() (regsync.Config, error) {
	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(config.Repositories)),
		Defaults: regsync.ConfigDefaults{
			UserAgent: "rancher-image-mirror",
		},
		Sync: make([]regsync.ConfigSync, 0),
	}
	for _, targetRepository := range config.Repositories {
		credEntry := regsync.ConfigCred{
			Pass:          targetRepository.Password,
			Registry:      targetRepository.Registry,
			RepoAuth:      targetRepository.RepoAuth,
			ReqConcurrent: targetRepository.ReqConcurrent,
			User:          targetRepository.Username,
		}
		regsyncYaml.Creds = append(regsyncYaml.Creds, credEntry)
	}
	for _, image := range config.Images {
		for _, repo := range config.Repositories {
			if !repo.Target {
				continue
			}
			// source and destination images are the same
			if image.SourceImage == repo.BaseUrl+"/"+image.TargetImageName() {
				continue
			}
			syncEntries, err := image.ToRegsyncImages(repo)
			if err != nil {
				return regsync.Config{}, fmt.Errorf("failed to convert Image with SourceImage %q: %w", image.SourceImage, err)
			}
			regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
		}
	}
	return regsyncYaml, nil
}

func compareRepositories(a, b Repository) int {
	return strings.Compare(a.BaseUrl, b.BaseUrl)
}
