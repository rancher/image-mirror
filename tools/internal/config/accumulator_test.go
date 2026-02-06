package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactAccumulator(t *testing.T) {
	t.Run("AddArtifact", func(t *testing.T) {
		t.Run("should correctly accumulate multiple artifacts", func(t *testing.T) {
			artifact1, err := NewArtifact("test-org/artifact1", []string{"test1"}, "", nil, nil)
			assert.NoError(t, err)
			artifact2, err := NewArtifact("test-org/artifact2", []string{"test2"}, "", nil, nil)
			assert.NoError(t, err)

			accumulator := NewArtifactAccumulator()
			accumulator.AddArtifacts(artifact1, artifact2)

			artifacts := accumulator.Artifacts()
			assert.Len(t, artifacts, 2)
			assert.Contains(t, artifacts, artifact1)
			assert.Contains(t, artifacts, artifact2)
		})
	})

	t.Run("TagDifference", func(t *testing.T) {
		t.Run("should return the passed artifact if no artifact like it is present in accumulator", func(t *testing.T) {
			artifact, err := NewArtifact("test-org/artifact", []string{"qwer"}, "", nil, nil)
			assert.Nil(t, err)
			accumulator := NewArtifactAccumulator()
			diffArtifact, err := accumulator.TagDifference(artifact)
			assert.Nil(t, err)
			assert.Equal(t, artifact.DoNotMirror, diffArtifact.DoNotMirror)
			assert.Equal(t, artifact.SourceArtifact, diffArtifact.SourceArtifact)
			assert.Equal(t, artifact.TargetArtifactName(), diffArtifact.TargetArtifactName())
			assert.Equal(t, artifact.Tags, diffArtifact.Tags)
			assert.Equal(t, artifact.TargetRepositories, diffArtifact.TargetRepositories)
		})

		t.Run("should return the tags that are not already present in the accumulator", func(t *testing.T) {
			artifact1, err := NewArtifact("test-org/artifact", []string{"qwer"}, "", nil, nil)
			assert.Nil(t, err)
			accumulator := NewArtifactAccumulator()
			accumulator.AddArtifacts(artifact1)
			artifact2, err := NewArtifact("test-org/artifact", []string{"asdf", "qwer"}, "", nil, nil)
			assert.Nil(t, err)
			diffArtifact, err := accumulator.TagDifference(artifact2)
			assert.Nil(t, err)
			assert.Equal(t, diffArtifact.Tags, []string{"asdf"})
		})

		t.Run("should return nil for artifact if all tags are accounted for", func(t *testing.T) {
			artifact1, err := NewArtifact("test-org/artifact", []string{"qwer"}, "", nil, nil)
			assert.Nil(t, err)
			artifact2, err := NewArtifact("test-org/artifact", []string{"qwer"}, "", nil, nil)
			assert.Nil(t, err)
			accumulator := NewArtifactAccumulator()
			accumulator.AddArtifacts(artifact1)
			diffArtifact, err := accumulator.TagDifference(artifact2)
			assert.Nil(t, err)
			assert.Nil(t, diffArtifact)
		})
	})
}
