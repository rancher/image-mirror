package autoupdate

import (
	"testing"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConfigEntry(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message       string
			ConfigEntry   ConfigEntry
			ExpectedError string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid ConfigEntry with GithubRelease",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					GithubRelease: &GithubRelease{
						Owner:      "test-owner",
						Repository: "test-repo",
						Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return nil for a valid ConfigEntry with HelmLatest",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					HelmLatest: &HelmLatest{
						HelmRepo: "https://helm.cilium.io",
						Charts: map[string]map[string]Environment{
							"cilium": {
								"default": {"hubble.enabled=true"},
							},
						},
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return nil for a valid ConfigEntry with GithubTaggedImagesFile",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					GithubTaggedImagesFile: &GithubTaggedImagesFile{
						Owner:             "longhorn",
						Repository:        "longhorn",
						ImagesFilePath:    "deploy/longhorn-images.txt",
						VersionConstraint: ">=1.4.0",
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return nil for a valid ConfigEntry with Registry",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					Registry: &Registry{
						Images:        []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
						Latest:        false,
						VersionFilter: "^v1\\.([3-9][0-9])\\.[0-9]+$",
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return error when Name is not present",
				ConfigEntry: ConfigEntry{
					Name: "",
					GithubRelease: &GithubRelease{
						Owner:      "test-owner",
						Repository: "test-repo",
						Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					},
				},
				ExpectedError: "must specify Name",
			},
			{
				Message: "should return error when no autoupdate strategy is present",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
				},
				ExpectedError: "must specify an autoupdate strategy",
			},
			{
				Message: "should return error when multiple autoupdate strategies are present",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					GithubRelease: &GithubRelease{
						Owner:      "test-owner",
						Repository: "test-repo",
						Images: []AutoupdateImageRef{{
							SourceImage: "rancher/rancher",
						}},
					},
					HelmLatest: &HelmLatest{
						HelmRepo: "https://helm.cilium.io",
						Charts: map[string]map[string]Environment{
							"cilium": {
								"default": {"hubble.enabled=true"},
							},
						},
					},
				},
				ExpectedError: "must specify only one autoupdate strategy",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.ConfigEntry.Validate()
				if testCase.ExpectedError == "" {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}

func TestGetBranchHash(t *testing.T) {
	t.Run("should produce the same hash with the same set of images, but with different image order", func(t *testing.T) {
		image1, err := config.NewImage("test-org/image1", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)
		image2, err := config.NewImage("test-org/image2", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)

		imageSet1 := []*config.Image{image1, image2}
		hash1, err := hashImageSet(imageSet1)
		assert.Nil(t, err)

		imageSet2 := []*config.Image{image2, image1}
		hash2, err := hashImageSet(imageSet2)
		assert.Nil(t, err)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("should produce the same hash with the same image, but a different order of tags", func(t *testing.T) {
		image1, err := config.NewImage("test-org/image", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)
		images1 := []*config.Image{image1}
		hash1, err := hashImageSet(images1)
		assert.Nil(t, err)

		image2, err := config.NewImage("test-org/image", []string{"qwer", "asdf"}, "", nil)
		assert.Nil(t, err)
		images2 := []*config.Image{image2}
		hash2, err := hashImageSet(images2)
		assert.Nil(t, err)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("should produce the same hash with the same set of images", func(t *testing.T) {
		image1, err := config.NewImage("test-org/image1", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)
		image2, err := config.NewImage("test-org/image2", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)

		imageSet1 := []*config.Image{image1, image2}
		hash1, err := hashImageSet(imageSet1)
		assert.Nil(t, err)

		imageSet2 := []*config.Image{image1, image2}
		hash2, err := hashImageSet(imageSet2)
		assert.Nil(t, err)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("should produce a different hash with different set of tags", func(t *testing.T) {
		image1, err := config.NewImage("test-org/image", []string{"asdf", "qwer"}, "", nil)
		assert.Nil(t, err)
		images1 := []*config.Image{image1}
		hash1, err := hashImageSet(images1)
		assert.Nil(t, err)

		image2, err := config.NewImage("test-org/image", []string{"asdf", "qwer", "zxcv"}, "", nil)
		assert.Nil(t, err)
		images2 := []*config.Image{image2}
		hash2, err := hashImageSet(images2)
		assert.Nil(t, err)

		assert.NotEqual(t, hash1, hash2)
	})
}
