package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

type Config struct {
	Images       []Image
	Repositories []Repository
}

type Image struct {
	// The source image without any tags.
	SourceImage string
	// Used to specify the desired name of the image on the target. If it
	// is not specified, defaults to mirrored-<repoName>-<imageName>.
	TargetImageName string
	// The tags that we want to mirror.
	Tags []string
}

type Repository struct {
	BaseUrl      string
	EnvVarPrefix string
	// Whether the repository should have images mirrored to it.
	Target bool
}

func Parse(fileName string) (Config, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read: %w", err)
	}

	config := Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal as JSON: %w", err)
	}

	return config, nil
}
