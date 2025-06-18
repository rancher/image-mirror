package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"
)

// Image should not be instantiated directly. Instead, use NewImage().
type Image struct {
	// If true, the Image is not mirrored i.e. it is not added to the
	// regsync config when the regsync config is generated.
	DoNotMirror any `json:",omitempty"`
	// The source image without any tags.
	SourceImage            string
	defaultTargetImageName string
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

func (image *Image) doNotMirrorTag(tag string) (bool, error) {
	switch val := image.DoNotMirror.(type) {
	case nil:
		return false, nil
	case bool:
		return val, nil
	case []any:
		for _, valPart := range val {
			blacklistedTag, ok := valPart.(string)
			if !ok {
				return false, fmt.Errorf("failed to cast %v to string", valPart)
			}
			if blacklistedTag == tag {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

// ToRegsyncImages converts image into one ConfigSync (i.e. an image
// for regsync to sync) for each tag present in image. repo provides
// the target repository for each ConfigSync.
func (image *Image) ToRegsyncImages(repo Repository) ([]regsync.ConfigSync, error) {
	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		doNotMirror, err := image.doNotMirrorTag(tag)
		if err != nil {
			return nil, fmt.Errorf("failed to determine whether tag %q should be mirrored: %w", tag, err)
		}
		if doNotMirror {
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
