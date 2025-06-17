package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageAccumulator(t *testing.T) {
	t.Run("AddImage", func(t *testing.T) {
		t.Run("should correctly accumulate multiple images", func(t *testing.T) {
			image1, err := NewImage("test-org/image1", []string{"test1"})
			assert.NoError(t, err)
			image2, err := NewImage("test-org/image2", []string{"test2"})
			assert.NoError(t, err)

			accumulator := NewImageAccumulator()
			accumulator.AddImages(image1, image2)

			images := accumulator.Images()
			assert.Len(t, images, 2)
			assert.Contains(t, images, image1)
			assert.Contains(t, images, image2)
		})
	})

	t.Run("TagDifference", func(t *testing.T) {
		t.Run("should return the passed image if no image like it is present in accumulator", func(t *testing.T) {
			image, err := NewImage("test-org/image", []string{"qwer"})
			assert.Nil(t, err)
			accumulator := NewImageAccumulator()
			diffImage, err := accumulator.TagDifference(image)
			assert.Nil(t, err)
			assert.Equal(t, image.DoNotMirror, diffImage.DoNotMirror)
			assert.Equal(t, image.SourceImage, diffImage.SourceImage)
			assert.Equal(t, image.SpecifiedTargetImageName, diffImage.SpecifiedTargetImageName)
			assert.Equal(t, image.Tags, diffImage.Tags)
		})

		t.Run("should return the tags that are not already present in the accumulator", func(t *testing.T) {
			image1, err := NewImage("test-org/image", []string{"qwer"})
			assert.Nil(t, err)
			accumulator := NewImageAccumulator()
			accumulator.AddImages(image1)
			image2, err := NewImage("test-org/image", []string{"asdf", "qwer"})
			assert.Nil(t, err)
			diffImage, err := accumulator.TagDifference(image2)
			assert.Nil(t, err)
			assert.Equal(t, diffImage.Tags, []string{"asdf"})
		})

		t.Run("should return nil for image if all tags are accounted for", func(t *testing.T) {
			image1, err := NewImage("test-org/image", []string{"qwer"})
			assert.Nil(t, err)
			image2, err := NewImage("test-org/image", []string{"qwer"})
			assert.Nil(t, err)
			accumulator := NewImageAccumulator()
			accumulator.AddImages(image1)
			diffImage, err := accumulator.TagDifference(image2)
			assert.Nil(t, err)
			assert.Nil(t, diffImage)
		})
	})
}
