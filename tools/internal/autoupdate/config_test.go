package autoupdate

import (
	"testing"

	"github.com/google/go-github/v71/github"
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

func TestNewReviewersRequest(t *testing.T) {
	type testCase struct {
		Name      string
		Reviewers []string
		Expected  github.ReviewersRequest
	}
	testCases := []testCase{
		{
			Name:      "should correctly parse users and teams",
			Reviewers: []string{"user1", "my-org/team-one", "user2", "another-org/team-two"},
			Expected: github.ReviewersRequest{
				Reviewers:     []string{"user1", "user2"},
				TeamReviewers: []string{"team-one", "team-two"},
			},
		},
		{
			Name:      "should handle only users",
			Reviewers: []string{"user1", "user2"},
			Expected: github.ReviewersRequest{
				Reviewers: []string{"user1", "user2"},
			},
		},
		{
			Name:      "should handle only teams",
			Reviewers: []string{"my-org/team-one", "another-org/team-two"},
			Expected: github.ReviewersRequest{
				TeamReviewers: []string{"team-one", "team-two"},
			},
		},
		{
			Name:      "should handle empty list",
			Reviewers: []string{},
			Expected:  github.ReviewersRequest{},
		},
		{
			Name:      "should handle malformed team strings",
			Reviewers: []string{"user1", "org/"},
			Expected: github.ReviewersRequest{
				Reviewers:     []string{"user1"},
				TeamReviewers: []string{},
			},
		},
		{
			Name:      "should handle team strings with multiple slashes",
			Reviewers: []string{"org/team/foo"},
			Expected: github.ReviewersRequest{
				TeamReviewers: []string{"team/foo"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			req := newReviewersRequest(tc.Reviewers)
			assert.ElementsMatch(t, tc.Expected.Reviewers, req.Reviewers)
			assert.ElementsMatch(t, tc.Expected.TeamReviewers, req.TeamReviewers)
		})
	}
}
