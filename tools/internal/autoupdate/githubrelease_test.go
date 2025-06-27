package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGithubRelease(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message       string
			GithubRelease *GithubRelease
			ExpectedError string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid GithubRelease using LatestOnly",
				GithubRelease: &GithubRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					LatestOnly: true,
				},
				ExpectedError: "",
			},
			{
				Message: "should return nil for a valid GithubRelease using VersionConstraint",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionConstraint: ">3.5.10",
				},
				ExpectedError: "",
			},
			{
				Message: "should return error if using both LatestOnly and VersionConstraint",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					LatestOnly:        true,
					VersionConstraint: ">3.5.10",
				},
				ExpectedError: "must not specify VersionConstraint when LatestOnly=true",
			},
			{
				Message: "should return error for empty Owner",
				GithubRelease: &GithubRelease{
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				ExpectedError: "must specify Owner",
			},
			{
				Message: "should return error for empty Repository",
				GithubRelease: &GithubRelease{
					Owner:  "test-owner",
					Images: []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				ExpectedError: "must specify Repository",
			},
			{
				Message: "should return error for nil Images",
				GithubRelease: &GithubRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
				},
				ExpectedError: "must specify Images",
			},
			{
				Message: "should return error for empty Images",
				GithubRelease: &GithubRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{},
				},
				ExpectedError: "must specify at least one element for Images",
			},
			{
				Message: "should return error for invalid version constraint",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionConstraint: "InvalidVersionConstraint",
				},
				ExpectedError: "invalid VersionConstraint: improper constraint: InvalidVersionConstraint",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.GithubRelease.Validate()
				if testCase.ExpectedError == "" {
					assert.Nil(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
