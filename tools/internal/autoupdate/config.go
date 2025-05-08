package autoupdate

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/config"

	"sigs.k8s.io/yaml"
)

type ConfigEntry struct {
	Name                string
	GithubLatestRelease *GithubLatestRelease
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
	if entry.GithubLatestRelease == nil {
		return errors.New("must specify an autoupdate strategy")
	}
	return nil
}

func (entry ConfigEntry) GetLatestImages() ([]*config.Image, error) {
	switch {
	case entry.GithubLatestRelease != nil:
		return entry.GithubLatestRelease.GetLatestImages()
	default:
		return nil, errors.New("did not find update strategy")
	}
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
