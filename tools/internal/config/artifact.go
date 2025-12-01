package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/rancher/artifact-mirror/internal/regsync"
)

// Artifact should not be instantiated directly. Instead, use NewArtifact().
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
	// Used to specify the desired name of the target artifact if it differs
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

func NewArtifact(sourceArtifact string, tags []string, targetArtifactName string, doNotMirror any, targetRepositories []string) (*Artifact, error) {
	artifact := &Artifact{
		SourceArtifact:     sourceArtifact,
		Tags:               tags,
		DoNotMirror:        doNotMirror,
		TargetRepositories: targetRepositories,
	}
	if err := artifact.setDefaults(); err != nil {
		return nil, err
	}
	artifact.SetTargetArtifactName(targetArtifactName)
	return artifact, nil
}

func (artifact *Artifact) Sort() {
	slices.Sort(artifact.Tags)
}

func (artifact *Artifact) setDefaults() error {
	parts := strings.Split(artifact.SourceArtifact, "/")
	if len(parts) < 2 {
		return fmt.Errorf("source artifact split into %d parts (>=2 parts expected)", len(parts))
	}
	if parts[0] == "dp.apps.rancher.io" {
		// AppCo artifacts have only one significant part in their reference.
		// For example, in dp.apps.rancher.io/containers/openjdk,
		// dp.apps.rancher.io/containers is the repository and openjdk is
		// the significant part.
		artifactName := parts[len(parts)-1]
		artifact.defaultTargetArtifactName = "appco-" + artifactName
	} else {
		repoName := parts[len(parts)-2]
		artifactName := parts[len(parts)-1]
		artifact.defaultTargetArtifactName = "mirrored-" + repoName + "-" + artifactName
	}

	artifact.excludeAllTags = false
	artifact.excludedTags = map[string]struct{}{}
	switch val := artifact.DoNotMirror.(type) {
	case nil:
	case bool:
		artifact.excludeAllTags = val
	case []any:
		for _, valPart := range val {
			excludedTag, ok := valPart.(string)
			if !ok {
				return fmt.Errorf("failed to cast %v to string", valPart)
			}
			if _, present := artifact.excludedTags[excludedTag]; present {
				return fmt.Errorf("DoNotMirror entry %q is duplicated", excludedTag)
			}
			artifact.excludedTags[excludedTag] = struct{}{}
		}
	default:
		return errors.New("DoNotMirror must be nil, bool, or []any")
	}

	if artifact.TargetRepositories == nil {
		artifact.TargetRepositories = []string{}
	}

	return nil
}

func (artifact *Artifact) TargetArtifactName() string {
	if artifact.SpecifiedTargetArtifactName != "" {
		return artifact.SpecifiedTargetArtifactName
	}
	return artifact.defaultTargetArtifactName
}

func (artifact *Artifact) SetTargetArtifactName(value string) {
	if value == artifact.defaultTargetArtifactName {
		artifact.SpecifiedTargetArtifactName = ""
	} else {
		artifact.SpecifiedTargetArtifactName = value
	}
}

func (artifact *Artifact) CombineSourceArtifactAndTags() []string {
	fullArtifacts := make([]string, 0, len(artifact.Tags))
	for _, tag := range artifact.Tags {
		fullArtifact := artifact.SourceArtifact + ":" + tag
		fullArtifacts = append(fullArtifacts, fullArtifact)
	}
	return fullArtifacts
}

// ToRegsyncArtifacts converts artifact into one ConfigSync (i.e. an artifact
// for regsync to sync) for each tag present in artifact, for each repository
// passed in repositories.
func (artifact *Artifact) ToRegsyncArtifacts(repositories []Repository) ([]regsync.ConfigSync, error) {
	entries := make([]regsync.ConfigSync, 0)
	for _, repository := range repositories {
		if !repository.DefaultTarget && len(artifact.TargetRepositories) == 0 {
			continue
		}
		if len(artifact.TargetRepositories) > 0 && !slices.Contains(artifact.TargetRepositories, repository.BaseUrl) {
			continue
		}
		// do not include if source and destination artifacts are the same
		trimmedSourceArtifact := strings.TrimPrefix(artifact.SourceArtifact, "docker.io/")
		trimmedTargetArtifact := strings.TrimPrefix(repository.BaseUrl+"/"+artifact.TargetArtifactName(), "docker.io/")
		if trimmedSourceArtifact == trimmedTargetArtifact {
			continue
		}
		syncEntries, err := artifact.ToRegsyncArtifactsForSingleRepository(repository)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Artifact with SourceArtifact %q: %w", artifact.SourceArtifact, err)
		}
		entries = append(entries, syncEntries...)
	}
	return entries, nil
}

// ToRegsyncArtifactsForSingleRepository converts artifact into one ConfigSync
// (i.e. an artifact for regsync to sync) for each tag present in artifact.
// repo provides the target repository for each ConfigSync.
func (artifact *Artifact) ToRegsyncArtifactsForSingleRepository(repo Repository) ([]regsync.ConfigSync, error) {
	if artifact.excludeAllTags {
		return nil, nil
	}
	entries := make([]regsync.ConfigSync, 0, len(artifact.Tags))
	for _, tag := range artifact.Tags {
		if _, excluded := artifact.excludedTags[tag]; excluded {
			continue
		}
		sourceArtifact := artifact.SourceArtifact + ":" + tag
		targetArtifact := repo.BaseUrl + "/" + artifact.TargetArtifactName() + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceArtifact,
			Target: targetArtifact,
			Type:   "image", //This works for both images and helm charts. More info on https://regclient.org/usage/regsync/#sync
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (artifact *Artifact) DeepCopy() *Artifact {
	copiedArtifact := &Artifact{
		DoNotMirror:                 artifact.DoNotMirror,
		SourceArtifact:              artifact.SourceArtifact,
		defaultTargetArtifactName:   artifact.defaultTargetArtifactName,
		SpecifiedTargetArtifactName: artifact.SpecifiedTargetArtifactName,
		excludeAllTags:              artifact.excludeAllTags,
		excludedTags:                maps.Clone(artifact.excludedTags),
		Tags:                        slices.Clone(artifact.Tags),
		TargetRepositories:          slices.Clone(artifact.TargetRepositories),
	}
	return copiedArtifact
}

func CompareArtifacts(a, b *Artifact) int {
	if sourceArtifactValue := strings.Compare(a.SourceArtifact, b.SourceArtifact); sourceArtifactValue != 0 {
		return sourceArtifactValue
	}
	return strings.Compare(a.TargetArtifactName(), b.TargetArtifactName())
}
