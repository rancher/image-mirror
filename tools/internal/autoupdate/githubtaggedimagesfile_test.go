package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGithubTaggedImagesFile(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message                string
			GithubTaggedImagesFile *GithubTaggedImagesFile
			ExpectedError          string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid GithubTaggedImagesFile",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "longhorn",
					Repository:        "longhorn",
					ImagesFilePath:    "deploy/longhorn-images.txt",
					VersionConstraint: ">=1.4.0",
				},
				ExpectedError: "",
			},
			{
				Message: "should return nil for a valid GithubTaggedImagesFile with complex version constraint",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "test-owner",
					Repository:        "test-repo",
					ImagesFilePath:    "images.txt",
					VersionConstraint: ">=1.0.0, <2.0.0",
				},
				ExpectedError: "",
			},
			{
				Message: "should return error for empty Owner",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Repository:        "longhorn",
					ImagesFilePath:    "deploy/longhorn-images.txt",
					VersionConstraint: ">=1.4.0",
				},
				ExpectedError: "must specify Owner",
			},
			{
				Message: "should return error for empty Repository",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "longhorn",
					ImagesFilePath:    "deploy/longhorn-images.txt",
					VersionConstraint: ">=1.4.0",
				},
				ExpectedError: "must specify Repository",
			},
			{
				Message: "should return error for empty ImagesFilePath",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "longhorn",
					Repository:        "longhorn",
					VersionConstraint: ">=1.4.0",
				},
				ExpectedError: "must specify ImagesFilePath",
			},
			{
				Message: "should return nil for empty VersionConstraint",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:          "longhorn",
					Repository:     "longhorn",
					ImagesFilePath: "deploy/longhorn-images.txt",
				},
				ExpectedError: "",
			},
			{
				Message: "should return error for invalid VersionConstraint format",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "longhorn",
					Repository:        "longhorn",
					ImagesFilePath:    "deploy/longhorn-images.txt",
					VersionConstraint: "invalid-version-constraint",
				},
				ExpectedError: `invalid VersionConstraint "invalid-version-constraint": improper constraint: invalid-version-constraint`,
			},
			{
				Message: "should return error for malformed version constraint",
				GithubTaggedImagesFile: &GithubTaggedImagesFile{
					Owner:             "longhorn",
					Repository:        "longhorn",
					ImagesFilePath:    "deploy/longhorn-images.txt",
					VersionConstraint: ">>1.0.0",
				},
				ExpectedError: `invalid VersionConstraint ">>1.0.0": improper constraint: >>1.0.0`,
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.GithubTaggedImagesFile.Validate()
				if testCase.ExpectedError == "" {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
