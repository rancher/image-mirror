package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"
)

// Image should not be instantiated directly. Instead, use NewImage().
type Image struct {
	// If DoNotMirror is a bool and true, the Image is not mirrored i.e.
	// it is not added to the regsync config when the regsync config is
	// generated. If DoNotMirror is a slice of strings, it specifies tags
	// that are not to be mirrored. Other types are invalid.
	DoNotMirror any `json:",omitempty"`
	// The source image without any tags.
	SourceImage            string
	defaultTargetImageName string
	// Set via DoNotMirror.
	excludeAllTags bool
	// Set via DoNotMirror.
	excludedTags map[string]struct{}
	// Used to specify the desired name of the target image if it differs
	// from default. This field would be private if it was convenient for
	// marshalling to JSON/YAML, but it is not. This field should not be
	// accessed directly - instead, use the TargetImageName() and
	// SetTargetImageName() methods.
	SpecifiedTargetImageName string `json:"TargetImageName,omitempty"`
	// The tags that we want to mirror.
	Tags []string
}

func NewImage(sourceImage string, tags []string) (*Image, error) {
	image := &Image{
		SourceImage: sourceImage,
		Tags:        tags,
	}
	if err := image.setDefaults(); err != nil {
		return nil, err
	}
	return image, nil
}

func (image *Image) Sort() {
	slices.Sort(image.Tags)
}

func (image *Image) setDefaults() error {
	parts := strings.Split(image.SourceImage, "/")
	if len(parts) < 2 {
		return fmt.Errorf("source image split into %d parts (>=2 parts expected)", len(parts))
	}

	if parts[0] == "dp.apps.rancher.io" {
		// AppCo images have only one significant part in their reference.
		// For example, in dp.apps.rancher.io/containers/openjdk,
		// dp.apps.rancher.io/containers is the repository and openjdk is
		// the significant part.
		imageName := parts[len(parts)-1]
		image.defaultTargetImageName = "appco-" + imageName
	} else {
		repoName := parts[len(parts)-2]
		imageName := parts[len(parts)-1]
		image.defaultTargetImageName = "mirrored-" + repoName + "-" + imageName
	}

	image.excludeAllTags = false
	image.excludedTags = map[string]struct{}{}
	switch val := image.DoNotMirror.(type) {
	case nil:
	case bool:
		image.excludeAllTags = val
	case []any:
		for _, valPart := range val {
			excludedTag, ok := valPart.(string)
			if !ok {
				return fmt.Errorf("failed to cast %v to string", valPart)
			}
			_, present := image.excludedTags[excludedTag]
			if present {
				return fmt.Errorf("DoNotMirror entry %q is duplicated", excludedTag)
			}
			image.excludedTags[excludedTag] = struct{}{}
		}
	default:
		return errors.New("DoNotMirror must be nil, bool, or []any")
	}

	return nil
}

func (image *Image) TargetImageName() string {
	if image.SpecifiedTargetImageName != "" {
		return image.SpecifiedTargetImageName
	}
	return image.defaultTargetImageName
}

func (image *Image) SetTargetImageName(value string) {
	if value == image.defaultTargetImageName {
		image.SpecifiedTargetImageName = ""
	} else {
		image.SpecifiedTargetImageName = value
	}
}

func (image *Image) CombineSourceImageAndTags() []string {
	fullImages := make([]string, 0, len(image.Tags))
	for _, tag := range image.Tags {
		fullImage := image.SourceImage + ":" + tag
		fullImages = append(fullImages, fullImage)
	}
	return fullImages
}

// ToRegsyncImages converts image into one ConfigSync (i.e. an image
// for regsync to sync) for each tag present in image. repo provides
// the target repository for each ConfigSync.
func (image *Image) ToRegsyncImages(repo Repository) ([]regsync.ConfigSync, error) {
	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		if _, excluded := image.excludedTags[tag]; excluded || image.excludeAllTags {
			continue
		}
		sourceImage := image.SourceImage + ":" + tag
		targetImage := repo.BaseUrl + "/" + image.TargetImageName() + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceImage,
			Target: targetImage,
			Type:   "image",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
