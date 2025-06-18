package config

import (
	"testing"

	"github.com/rancher/image-mirror/internal/regsync"
	"github.com/stretchr/testify/assert"
)

func TestImage(t *testing.T) {
	t.Run("ToRegsyncImages", func(t *testing.T) {
		type TestCase struct {
			Name                     string
			SpecifiedTargetImageName string
			ExpectedEntries          []regsync.ConfigSync
		}
		for _, testCase := range []TestCase{
			{
				Name:                     "should use default image name when TargetImageName is not set",
				SpecifiedTargetImageName: "",
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-image:v1.2.3",
						Target: "docker.io/test1/mirrored-test-org-test-image:v1.2.3",
						Type:   "image",
					},
					{
						Source: "test-org/test-image:v2.3.4",
						Target: "docker.io/test1/mirrored-test-org-test-image:v2.3.4",
						Type:   "image",
					},
				},
			},
			{
				Name:                     "should use TargetImageName when it is set",
				SpecifiedTargetImageName: "other-org-test-image",
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-image:v1.2.3",
						Target: "docker.io/test1/other-org-test-image:v1.2.3",
						Type:   "image",
					},
					{
						Source: "test-org/test-image:v2.3.4",
						Target: "docker.io/test1/other-org-test-image:v2.3.4",
						Type:   "image",
					},
				},
			},
		} {
			t.Run(testCase.Name, func(t *testing.T) {
				inputImage, err := NewImage("test-org/test-image", []string{
					"v1.2.3",
					"v2.3.4",
				})
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				inputImage.SetTargetImageName(testCase.SpecifiedTargetImageName)
				inputRepository := Repository{
					BaseUrl: "docker.io/test1",
				}
				regsyncEntries, err := inputImage.ToRegsyncImages(inputRepository)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				assert.Len(t, regsyncEntries, len(testCase.ExpectedEntries))
				for _, expectedEntry := range testCase.ExpectedEntries {
					assert.Contains(t, regsyncEntries, expectedEntry)
				}
			})
		}
	})

	t.Run("DoNotMirrorTag", func(t *testing.T) {
		t.Run("should always return true when DoNotMirror is nil", func(t *testing.T) {
			image, err := NewImage("test/test", []string{"tag1"})
			assert.NoError(t, err)
			doNotMirror, err := image.doNotMirrorTag("tag1")
			assert.NoError(t, err)
			assert.False(t, doNotMirror)
		})

		t.Run("should return true when DoNotMirror is true", func(t *testing.T) {
			image, err := NewImage("test/test", []string{"tag1"})
			assert.NoError(t, err)
			image.DoNotMirror = true
			doNotMirror, err := image.doNotMirrorTag("tag1")
			assert.NoError(t, err)
			assert.True(t, doNotMirror)
		})

		t.Run("should return false when DoNotMirror is false", func(t *testing.T) {
			image, err := NewImage("test/test", []string{"tag1"})
			assert.NoError(t, err)
			image.DoNotMirror = false
			doNotMirror, err := image.doNotMirrorTag("tag1")
			assert.NoError(t, err)
			assert.False(t, doNotMirror)
		})

		t.Run("should return true when DoNotMirror is string slice and contains tag", func(t *testing.T) {
			image, err := NewImage("test/test", []string{"tag1"})
			assert.NoError(t, err)
			image.DoNotMirror = []any{"tag1"}
			doNotMirror, err := image.doNotMirrorTag("tag1")
			assert.NoError(t, err)
			assert.True(t, doNotMirror)
		})

		t.Run("should return false when DoNotMirror is string slice and does not contain tag", func(t *testing.T) {
			image, err := NewImage("test/test", []string{"tag1", "tag2"})
			assert.NoError(t, err)
			image.DoNotMirror = []any{"tag1"}
			doNotMirror, err := image.doNotMirrorTag("tag2")
			assert.NoError(t, err)
			assert.False(t, doNotMirror)
		})
	})
}
