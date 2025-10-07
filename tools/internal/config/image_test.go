package config

import (
	"strings"
	"testing"

	"github.com/rancher/image-mirror/internal/regsync"
	"github.com/stretchr/testify/assert"
)

func TestImage(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should exclude repos with Target: false from sync entries", func(t *testing.T) {
			image, err := NewImage("test-org/image1", []string{"v1.0.0"}, "", nil)
			assert.NoError(t, err)
			repositories := []Repository{
				{
					BaseUrl:  "docker.io/target-repo",
					Target:   true,
					Username: "target-user",
					Password: "target-pass",
					Registry: "docker.io",
				},
				{
					BaseUrl:  "docker.io/non-target-repo",
					Target:   false,
					Username: "non-target-user",
					Password: "non-target-pass",
					Registry: "docker.io",
				},
			}

			configEntries, err := image.ToRegsyncImages(repositories)
			assert.NoError(t, err)

			for _, configEntry := range configEntries {
				assert.True(t, strings.HasPrefix(configEntry.Target, repositories[0].BaseUrl))
			}
		})

		t.Run("should exclude images with same source and target", func(t *testing.T) {
			type TestCase struct {
				Name            string
				ImageRef        string
				BaseUrl         string
				ExpectedPresent bool
			}
			testCases := []TestCase{
				{
					Name:            "dockerhub image with docker.io prefix and dockerhub base URL",
					ImageRef:        "docker.io/test-org/test-image",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: false,
				},
				{
					Name:            "dockerhub image with docker.io prefix and non-dockerhub base URL",
					ImageRef:        "docker.io/test-org/test-image",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "dockerhub image without docker.io prefix and with dockerhub base URL",
					ImageRef:        "test-org/test-image",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: false,
				},
				{
					Name:            "dockerhub image without docker.io prefix and with non-dockerhub base URL",
					ImageRef:        "test-org/test-image",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "non-dockerhub image with dockerhub base URL",
					ImageRef:        "some.other.registry/test-org/test-image",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "non-dockerhub image with non-dockerhub base URL",
					ImageRef:        "some.other.registry/test-org/test-image",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: false,
				},
			}
			for _, testCase := range testCases {
				t.Run(testCase.Name, func(t *testing.T) {
					tag := "v1.0.0"
					image, err := NewImage(testCase.ImageRef, []string{tag}, "test-image", nil)
					assert.NoError(t, err)
					repositories := []Repository{
						{
							BaseUrl: testCase.BaseUrl,
							Target:  true,
						},
					}

					configEntries, err := image.ToRegsyncImages(repositories)
					assert.NoError(t, err)

					if testCase.ExpectedPresent {
						assert.Len(t, configEntries, 1)
						assert.Equal(t, image.SourceImage+":"+tag, configEntries[0].Source)
						assert.Equal(t, testCase.BaseUrl+"/"+image.TargetImageName()+":"+tag, configEntries[0].Target)
					} else {
						assert.Len(t, configEntries, 0)
					}
				})
			}
		})

		t.Run("should target all repositories when TargetRepositories is empty", func(t *testing.T) {
			tags := []string{"v1.0.0"}
			image, err := NewImage("test-org/image", tags, "", nil)
			assert.NoError(t, err)
			repositories := []Repository{
				{
					BaseUrl: "some.site/registry",
					Target:  true,
				},
				{
					BaseUrl: "some.other.site/registry",
					Target:  true,
				},
			}

			configEntries, err := image.ToRegsyncImages(repositories)
			assert.NoError(t, err)

			assert.Len(t, configEntries, len(tags)*len(repositories))
			for _, configEntry := range configEntries {
				matchesIndex0 := strings.HasPrefix(configEntry.Target, repositories[0].BaseUrl)
				matchesIndex1 := strings.HasPrefix(configEntry.Target, repositories[1].BaseUrl)
				assert.True(t, matchesIndex0 || matchesIndex1)
			}
		})

		t.Run("should target only specified repositories when TargetRepositories is specified", func(t *testing.T) {
			tags := []string{"v1.0.0"}
			image, err := NewImage("test-org/image", tags, "", nil)
			assert.NoError(t, err)
			targetedRepositories := []string{"some.site/registry"}
			image.TargetRepositories = targetedRepositories
			repositories := []Repository{
				{
					BaseUrl: targetedRepositories[0],
					Target:  true,
				},
				{
					BaseUrl: "some.other.site/registry",
					Target:  true,
				},
			}

			configEntries, err := image.ToRegsyncImages(repositories)
			assert.NoError(t, err)

			assert.Len(t, configEntries, len(tags)*len(targetedRepositories))
			for _, configEntry := range configEntries {
				assert.True(t, strings.HasPrefix(configEntry.Target, targetedRepositories[0]))
			}
		})
	})

	t.Run("ToRegsyncImagesForSingleRepository", func(t *testing.T) {
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
				regsyncEntries, err := inputImage.ToRegsyncImagesForSingleRepository(inputRepository)
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
