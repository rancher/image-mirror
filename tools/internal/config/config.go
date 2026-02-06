package config

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/rancher/artifact-mirror/internal/regsync"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Artifacts    []*Artifact
	Repositories []Repository
}

type Repository struct {
	// BaseUrl is used exclusively for referring to the Repository
	// in general, and for building the target artifact ref for a given
	// artifact for a repository. For example, a target artifact name of
	// "mirrored-rancher-cis-operator" and a BaseUrl of "docker.io/rancher"
	// produce a target artifact ref of "docker.io/rancher/mirrored-rancher-cis-operator".
	BaseUrl string
	// Whether the Repository is used as a target repository for a given
	// Artifact when the TargetRepositories field of the Artifact is not set.
	DefaultTarget bool
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
	return ParseFromBytes(contents)
}

func ParseFromBytes(contents []byte) (*Config, error) {
	config := &Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	for _, artifact := range config.Artifacts {
		if err := artifact.setDefaults(); err != nil {
			return nil, fmt.Errorf("failed to set defaults for artifact %q: %w", artifact.SourceArtifact, err)
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
	for _, artifact := range config.Artifacts {
		artifact.Sort()
	}
	slices.SortStableFunc(config.Artifacts, CompareArtifacts)
	slices.SortStableFunc(config.Repositories, compareRepositories)
}

func (config *Config) ToRegsyncConfig() (regsync.Config, error) {
	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(config.Repositories)),
		Defaults: regsync.ConfigDefaults{
			UserAgent: "rancher-artifact-mirror",
		},
		Sync: make([]regsync.ConfigSync, 0),
	}

	credsMap := map[regsync.ConfigCred]struct{}{}
	for _, targetRepository := range config.Repositories {
		credEntry := regsync.ConfigCred{
			Pass:          targetRepository.Password,
			Registry:      targetRepository.Registry,
			RepoAuth:      targetRepository.RepoAuth,
			ReqConcurrent: targetRepository.ReqConcurrent,
			User:          targetRepository.Username,
		}
		if _, ok := credsMap[credEntry]; ok {
			continue
		}
		credsMap[credEntry] = struct{}{}
	}
	regsyncYaml.Creds = slices.SortedStableFunc(maps.Keys(credsMap), func(a, b regsync.ConfigCred) int {
		return cmp.Compare(a.Registry, b.Registry)
	})

	for _, artifact := range config.Artifacts {
		syncEntries, err := artifact.ToRegsyncArtifacts(config.Repositories)
		if err != nil {
			return regsync.Config{}, fmt.Errorf("failed to convert Artifact with SourceArtifact %q: %w", artifact.SourceArtifact, err)
		}
		regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
	}

	return regsyncYaml, nil
}

func (config *Config) DeepCopy() *Config {
	copiedConfig := &Config{
		Artifacts:    make([]*Artifact, 0, len(config.Artifacts)),
		Repositories: slices.Clone(config.Repositories),
	}
	for _, artifact := range config.Artifacts {
		copiedConfig.Artifacts = append(copiedConfig.Artifacts, artifact.DeepCopy())
	}
	return copiedConfig
}

func compareRepositories(a, b Repository) int {
	return strings.Compare(a.BaseUrl, b.BaseUrl)
}
