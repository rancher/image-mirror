package config

import (
	"fmt"
	"slices"
)

type ArtifactIndex struct {
	SourceArtifact     string
	TargetArtifactName string
}

type ArtifactAccumulator struct {
	mapping map[ArtifactIndex]*Artifact
}

func NewArtifactAccumulator() *ArtifactAccumulator {
	return &ArtifactAccumulator{
		mapping: map[ArtifactIndex]*Artifact{},
	}
}

func (ia *ArtifactAccumulator) AddArtifacts(newArtifacts ...*Artifact) {
	for _, newArtifact := range newArtifacts {
		pair := ArtifactIndex{
			SourceArtifact:     newArtifact.SourceArtifact,
			TargetArtifactName: newArtifact.TargetArtifactName(),
		}
		existingArtifact, ok := ia.mapping[pair]
		if !ok {
			ia.mapping[pair] = newArtifact
		} else {
			for _, newTag := range newArtifact.Tags {
				if !slices.Contains(existingArtifact.Tags, newTag) {
					existingArtifact.Tags = append(existingArtifact.Tags, newTag)
				}
			}
			ia.mapping[pair] = existingArtifact
		}
	}
}

// TagDifference returns an Artifact containing the tags of artifact that are
// not already accounted for in the artifact accumulator. If all tags of
// the artifact are accounted for, nil is returned for the artifact. The
// "difference" terminology comes from set theory.
func (ia *ArtifactAccumulator) TagDifference(artifact *Artifact) (*Artifact, error) {
	index := ArtifactIndex{
		SourceArtifact:     artifact.SourceArtifact,
		TargetArtifactName: artifact.TargetArtifactName(),
	}
	existingArtifact, ok := ia.mapping[index]
	if !ok {
		return artifact, nil
	}

	artifactToReturn, err := NewArtifact(artifact.SourceArtifact, make([]string, 0, len(artifact.Tags)), artifact.TargetArtifactName(), artifact.DoNotMirror, artifact.TargetRepositories)
	if err != nil {
		return nil, fmt.Errorf("failed to construct new artifact from passed artifact: %w", err)
	}
	for _, tag := range artifact.Tags {
		if !slices.Contains(existingArtifact.Tags, tag) {
			artifactToReturn.Tags = append(artifactToReturn.Tags, tag)
		}
	}
	if len(artifactToReturn.Tags) == 0 {
		return nil, nil
	}
	return artifactToReturn, nil
}

func (ia *ArtifactAccumulator) Artifacts() []*Artifact {
	artifacts := make([]*Artifact, 0, len(ia.mapping))
	for _, artifact := range ia.mapping {
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func (ia *ArtifactAccumulator) Contains(artifact *Artifact) bool {
	index := ArtifactIndex{
		SourceArtifact:     artifact.SourceArtifact,
		TargetArtifactName: artifact.TargetArtifactName(),
	}
	_, ok := ia.mapping[index]
	return ok
}
