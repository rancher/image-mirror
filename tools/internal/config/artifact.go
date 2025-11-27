package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/regsync"
)

// Artifact should not be instantiated directly. Instead, use NewImage().
type Artifact struct {
	// If DoNotMirror is a bool and true, the Artifact is not mirrored i.e.
	// it is not added to the regsync config when the regsync config is
	// generated. If DoNotMirror is a slice of strings, it specifies tags
	// that are not to be mirrored. Other types are invalid.
	DoNotMirror any `json:",omitempty"`
	// The source artifact without any tags.
	SourceArtifact            string
	defaultTargetArtifactName string
	// Set via DoNotMirror.
	excludeAllTags bool
	// Set via DoNotMirror.
	excludedTags map[string]struct{}
	// Used to specify the desired name of the target image if it differs
	// from default. This field would be private if it was convenient for
	// marshalling to JSON/YAML, but it is not. This field should not be
	// accessed directly - instead, use the TargetArtifactName() and
	// SetTargetArtifactName() methods.
	SpecifiedTargetArtifactName string `json:"TargetArtifactName,omitempty"`
	// The tags that we want to mirror.
	Tags []string
	// The Repositories that you want to mirror the Artifact to. Repositories
	// are specified via their BaseUrl field. If TargetRepositories is not
	// specified, the Artifact is mirrored to all Repositories that have
	// DefaultTarget set to true.
	TargetRepositories []string `json:",omitempty"`
}

func NewImage(sourceImage string, tags []string, targetImageName string, doNotMirror any, targetRepositories []string) (*Artifact, error) {
	image := &Artifact{
		SourceArtifact:     sourceImage,
		Tags:               tags,
		DoNotMirror:        doNotMirror,
		TargetRepositories: targetRepositories,
	}
	if err := image.setDefaults(); err != nil {
		return nil, err
	}
	image.SetTargetArtifactName(targetImageName)
	return image, nil
}

func (image *Artifact) Sort() {
	slices.Sort(image.Tags)
}

func (image *Artifact) setDefaults() error {
	parts := strings.Split(image.SourceArtifact, "/")
	if len(parts) < 2 {
		return fmt.Errorf("source image split into %d parts (>=2 parts expected)", len(parts))
	}
	if parts[0] == "dp.apps.rancher.io" {
		// AppCo images have only one significant part in their reference.
		// For example, in dp.apps.rancher.io/containers/openjdk,
		// dp.apps.rancher.io/containers is the repository and openjdk is
		// the significant part.
		imageName := parts[len(parts)-1]
		image.defaultTargetArtifactName = "appco-" + imageName
	} else {
		repoName := parts[len(parts)-2]
		imageName := parts[len(parts)-1]
		image.defaultTargetArtifactName = "mirrored-" + repoName + "-" + imageName
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
			if _, present := image.excludedTags[excludedTag]; present {
				return fmt.Errorf("DoNotMirror entry %q is duplicated", excludedTag)
			}
			image.excludedTags[excludedTag] = struct{}{}
		}
	default:
		return errors.New("DoNotMirror must be nil, bool, or []any")
	}

	if image.TargetRepositories == nil {
		image.TargetRepositories = []string{}
	}

	return nil
}

func (image *Artifact) TargetArtifactName() string {
	if image.SpecifiedTargetArtifactName != "" {
		return image.SpecifiedTargetArtifactName
	}
	return image.defaultTargetArtifactName
}

func (image *Artifact) SetTargetArtifactName(value string) {
	if value == image.defaultTargetArtifactName {
		image.SpecifiedTargetArtifactName = ""
	} else {
		image.SpecifiedTargetArtifactName = value
	}
}

func (image *Artifact) CombineSourceImageAndTags() []string {
	fullImages := make([]string, 0, len(image.Tags))
	for _, tag := range image.Tags {
		fullImage := image.SourceArtifact + ":" + tag
		fullImages = append(fullImages, fullImage)
	}
	return fullImages
}

// ToRegsyncImages converts image into one ConfigSync (i.e. an image
// for regsync to sync) for each tag present in image, for each repository
// passed in repositories.
func (image *Artifact) ToRegsyncImages(repositories []Repository) ([]regsync.ConfigSync, error) {
	entries := make([]regsync.ConfigSync, 0)
	for _, repository := range repositories {
		if !repository.DefaultTarget && len(image.TargetRepositories) == 0 {
			continue
		}
		if len(image.TargetRepositories) > 0 && !slices.Contains(image.TargetRepositories, repository.BaseUrl) {
			continue
		}
		// do not include if source and destination images are the same
		trimmedSourceImage := strings.TrimPrefix(image.SourceArtifact, "docker.io/")
		trimmedTargetImage := strings.TrimPrefix(repository.BaseUrl+"/"+image.TargetArtifactName(), "docker.io/")
		if trimmedSourceImage == trimmedTargetImage {
			continue
		}
		syncEntries, err := image.ToRegsyncImagesForSingleRepository(repository)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Artifact with SourceArtifact %q: %w", image.SourceArtifact, err)
		}
		entries = append(entries, syncEntries...)
	}
	return entries, nil
}

// ToRegsyncImagesForSingleRepository converts image into one ConfigSync
// (i.e. an image for regsync to sync) for each tag present in image.
// repo provides the target repository for each ConfigSync.
func (image *Artifact) ToRegsyncImagesForSingleRepository(repo Repository) ([]regsync.ConfigSync, error) {
	if image.excludeAllTags {
		return nil, nil
	}
	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		if _, excluded := image.excludedTags[tag]; excluded {
			continue
		}
		sourceImage := image.SourceArtifact + ":" + tag
		targetImage := repo.BaseUrl + "/" + image.TargetArtifactName() + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceImage,
			Target: targetImage,
			Type:   "image",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (image *Artifact) DeepCopy() *Artifact {
	copiedImage := &Artifact{
		DoNotMirror:                 image.DoNotMirror,
		SourceArtifact:              image.SourceArtifact,
		defaultTargetArtifactName:   image.defaultTargetArtifactName,
		SpecifiedTargetArtifactName: image.SpecifiedTargetArtifactName,
		excludeAllTags:              image.excludeAllTags,
		excludedTags:                maps.Clone(image.excludedTags),
		Tags:                        slices.Clone(image.Tags),
		TargetRepositories:          slices.Clone(image.TargetRepositories),
	}
	return copiedImage
}

func CompareImages(a, b *Artifact) int {
	if sourceImageValue := strings.Compare(a.SourceArtifact, b.SourceArtifact); sourceImageValue != 0 {
		return sourceImageValue
	}
	return strings.Compare(a.TargetArtifactName(), b.TargetArtifactName())
}
