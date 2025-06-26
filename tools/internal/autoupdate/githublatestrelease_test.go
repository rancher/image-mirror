package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGithubLatestRelease(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message             string
			GithubLatestRelease *GithubLatestRelease
			ExpectedError       string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid GithubLatestRelease",
				GithubLatestRelease: &GithubLatestRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				ExpectedError: "",
			},
			{
				Message: "should return error for empty Owner",
				GithubLatestRelease: &GithubLatestRelease{
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				ExpectedError: "must specify Owner",
			},
			{
				Message: "should return error for empty Repository",
				GithubLatestRelease: &GithubLatestRelease{
					Owner:  "test-owner",
					Images: []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				ExpectedError: "must specify Repository",
			},
			{
				Message: "should return error for nil Images",
				GithubLatestRelease: &GithubLatestRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
				},
				ExpectedError: "must specify Images",
			},
			{
				Message: "should return error for empty Images",
				GithubLatestRelease: &GithubLatestRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{},
				},
				ExpectedError: "must specify at least one element for Images",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.GithubLatestRelease.Validate()
				if testCase.ExpectedError == "" {
					assert.Nil(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
