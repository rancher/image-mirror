package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should exclude repos with Target: false from sync entries", func(t *testing.T) {
			image1, err := NewImage("test-org/image1", []string{"v1.0.0"})
			assert.NoError(t, err)
			image2, err := NewImage("test-org/image2", []string{"v2.0.0"})
			assert.NoError(t, err)
			config := &Config{
				Images: []*Image{image1, image2},
				Repositories: []Repository{
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
				},
			}

			regsyncYaml, err := config.ToRegsyncConfig()
			assert.NoError(t, err)

			// Non-target repos should still be included in regsync.yaml, since
			// they may be the source of some images.
			assert.Len(t, regsyncYaml.Creds, 2)

			for _, regsyncYamlImage := range regsyncYaml.Sync {
				assert.True(t, strings.HasPrefix(regsyncYamlImage.Target, config.Repositories[0].BaseUrl))
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
					image, err := NewImage(testCase.ImageRef, []string{tag})
					assert.NoError(t, err)
					image.SetTargetImageName("test-image")
					config := &Config{
						Images: []*Image{image},
						Repositories: []Repository{
							{
								BaseUrl: testCase.BaseUrl,
								Target:  true,
							},
						},
					}
					regsyncYaml, err := config.ToRegsyncConfig()
					assert.NoError(t, err)
					if testCase.ExpectedPresent {
						assert.Len(t, regsyncYaml.Sync, 1)
						assert.Equal(t, image.SourceImage+":"+tag, regsyncYaml.Sync[0].Source)
						assert.Equal(t, testCase.BaseUrl+"/"+image.TargetImageName()+":"+tag, regsyncYaml.Sync[0].Target)
					} else {
						assert.Len(t, regsyncYaml.Sync, 0)
					}
				})
			}
		})
	})
}
