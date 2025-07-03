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
			DoNotMirror              any
			ExpectedEntries          []regsync.ConfigSync
		}
		for _, testCase := range []TestCase{
			{
				Name:                     "should use default image name when TargetImageName is not set",
				SpecifiedTargetImageName: "",
				DoNotMirror:              nil,
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
				DoNotMirror:              nil,
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
			{
				Name:                     "should not return any entries if DoNotMirror is true",
				SpecifiedTargetImageName: "",
				DoNotMirror:              true,
				ExpectedEntries:          []regsync.ConfigSync{},
			},
			{
				Name:                     "should only return unspecified tags if DoNotMirror specifies tags",
				SpecifiedTargetImageName: "",
				DoNotMirror:              []any{"v2.3.4"},
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-image:v1.2.3",
						Target: "docker.io/test1/mirrored-test-org-test-image:v1.2.3",
						Type:   "image",
					},
				},
			},
		} {
			t.Run(testCase.Name, func(t *testing.T) {
				inputImage, err := NewImage("test-org/test-image", []string{"v1.2.3", "v2.3.4"}, testCase.SpecifiedTargetImageName, testCase.DoNotMirror)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
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

	t.Run("setDefaults", func(t *testing.T) {
		t.Run("should return error for invalid DoNotMirror type", func(t *testing.T) {
			image := Image{
				SourceImage: "test/test",
				Tags:        []string{"tag1"},
				DoNotMirror: 1234,
			}
			err := image.setDefaults()
			assert.ErrorContains(t, err, "DoNotMirror must be nil, bool, or []any")
		})

		t.Run("should return error when DoNotMirror has invalid element type", func(t *testing.T) {
			image := Image{
				SourceImage: "test/test",
				Tags:        []string{"tag1"},
				DoNotMirror: []any{"asdf", 1234, "qwer123"},
			}
			err := image.setDefaults()
			assert.Errorf(t, err, "failed to cast %v to string", 1234)
		})

		t.Run("should return error when DoNotMirror has a duplicated element", func(t *testing.T) {
			image := Image{
				SourceImage: "test/test",
				Tags:        []string{"tag1"},
				DoNotMirror: []any{"asdf", "qwer", "asdf"},
			}
			err := image.setDefaults()
			assert.Error(t, err, "DoNotMirror entry asdf is duplicated")
		})

		t.Run("should return nil for valid DoNotMirror type", func(t *testing.T) {
			image := Image{
				SourceImage: "test/test",
				Tags:        []string{"tag1"},
			}
			doNotMirrorValues := []any{nil, true, []any{"tag1", "tag2"}}
			for _, doNotMirrorValue := range doNotMirrorValues {
				image.DoNotMirror = doNotMirrorValue
				err := image.setDefaults()
				assert.NoError(t, err)
			}
		})
	})

	t.Run("DeepCopy", func(t *testing.T) {
		t.Run("should copy all fields", func(t *testing.T) {
			original, err := NewImage("test-org/test-image", []string{"v1.0.0", "v2.0.0"}, "custom-image-name", []any{"v1.0.0"})
			assert.NoError(t, err)

			copy := original.DeepCopy()

			assert.Equal(t, original.DoNotMirror, copy.DoNotMirror)
			assert.Equal(t, original.SourceImage, copy.SourceImage)
			assert.Equal(t, original.defaultTargetImageName, copy.defaultTargetImageName)
			assert.Equal(t, original.SpecifiedTargetImageName, copy.SpecifiedTargetImageName)
			assert.Equal(t, original.Tags, copy.Tags)
			assert.Equal(t, original.excludeAllTags, copy.excludeAllTags)
			assert.Equal(t, original.excludedTags, copy.excludedTags)
		})
	})
}
