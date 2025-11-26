package config

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images     []*Image
	Registries []Registry
}

type Registry struct {
	// BaseUrl is used exclusively for referring to the Registry
	// in general, and for building the target image ref for a given
	// image for a registry. For example, a target image name of
	// "mirrored-rancher-cis-operator" and a BaseUrl of "docker.io/rancher"
	// produce a target image ref of "docker.io/rancher/mirrored-rancher-cis-operator".
	BaseUrl string
	// Whether the Registry is used as a target registry for a given
	// Image when the TargetRegistries field of the Image is not set.
	DefaultTarget bool
	// Password is what goes into the "pass" field of regsync.yaml
	// for this registry. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Password string
	// Registry is what goes into the "registry" field of regsync.yaml
	// for this registry. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	Registry string
	// RepoAuth goes into the "repoAuth" field of regsync.yaml in this
	// registry. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	RepoAuth bool `json:",omitempty"`
	// ReqConcurrent is what goes into the "reqConcurrent" field of
	// regsync.yaml for this registry. For more information please see
	// https://github.com/regclient/regclient/blob/main/docs/regsync.md
	ReqConcurrent int `json:",omitempty"`
	// Username is what goes into the "user" field of regsync.yaml
	// for this registry. For more information please see
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
	slices.SortStableFunc(config.Registries, compareRegistries)
}

func (config *Config) ToRegsyncConfig() (regsync.Config, error) {
	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(config.Registries)),
		Defaults: regsync.ConfigDefaults{
			UserAgent: "rancher-image-mirror",
		},
		Sync: make([]regsync.ConfigSync, 0),
	}

	credsMap := map[regsync.ConfigCred]struct{}{}
	for _, targetRegistry := range config.Registries {
		credEntry := regsync.ConfigCred{
			Pass:          targetRegistry.Password,
			Registry:      targetRegistry.Registry,
			RepoAuth:      targetRegistry.RepoAuth,
			ReqConcurrent: targetRegistry.ReqConcurrent,
			User:          targetRegistry.Username,
		}
		if _, ok := credsMap[credEntry]; ok {
			continue
		}
		credsMap[credEntry] = struct{}{}
	}
	regsyncYaml.Creds = slices.SortedStableFunc(maps.Keys(credsMap), func(a, b regsync.ConfigCred) int {
		return cmp.Compare(a.Registry, b.Registry)
	})

	for _, image := range config.Images {
		syncEntries, err := image.ToRegsyncImages(config.Registries)
		if err != nil {
			return regsync.Config{}, fmt.Errorf("failed to convert Image with SourceImage %q: %w", image.SourceImage, err)
		}
		regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
	}

	return regsyncYaml, nil
}

func (config *Config) DeepCopy() *Config {
	copiedConfig := &Config{
		Images:     make([]*Image, 0, len(config.Images)),
		Registries: slices.Clone(config.Registries),
	}
	for _, image := range config.Images {
		copiedConfig.Images = append(copiedConfig.Images, image.DeepCopy())
	}
	return copiedConfig
}

func compareRegistries(a, b Registry) int {
	return strings.Compare(a.BaseUrl, b.BaseUrl)
}
