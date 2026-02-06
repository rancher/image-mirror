package autoupdate

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message       string
			Registry      *Registry
			ExpectedError string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid Registry strategy",
				Registry: &Registry{
					Artifacts:     []AutoupdateArtifactRef{{SourceArtifact: "rancher/rancher"}},
					Latest:        true,
					VersionFilter: "^v1\\.([3-9][0-9])\\.[0-9]+$",
				},
				ExpectedError: "",
			},
			{
				Message: "should return error if no artifacts provided",
				Registry: &Registry{
					Artifacts:     nil,
					Latest:        true,
					VersionFilter: "^v1\\.([3-9][0-9])\\.[0-9]+$",
				},
				ExpectedError: "must specify at least one artifact",
			},
			{
				Message: "should return error the versionFilter provided is invalid",
				Registry: &Registry{
					Artifacts:     []AutoupdateArtifactRef{{SourceArtifact: "rancher/rancher"}},
					Latest:        false,
					VersionFilter: "[",
				},
				ExpectedError: "invalid version filter regex: error parsing regexp: missing closing ]: `[`",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.Registry.Validate()
				if testCase.ExpectedError == "" {
					assert.Nil(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
