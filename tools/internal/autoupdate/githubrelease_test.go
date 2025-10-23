package autoupdate

import (
	"regexp"
	"testing"

	"github.com/Masterminds/semver/v3"
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
			{
				Message: "should return error for invalid version regex",
				GithubRelease: &GithubRelease{
					Owner:        "test-owner",
					Repository:   "test-repo",
					Images:       []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionRegex: "v[asdf[",
				},
				ExpectedError: "invalid VersionRegex: error parsing regexp: missing closing ]: `[asdf[`",
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

		t.Run("should set compiledVersionConstraint", func(t *testing.T) {
			constraintString := ">=1.2.3"
			githubRelease := &GithubRelease{
				Owner:             "test-owner",
				Repository:        "test-repo",
				Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				VersionConstraint: constraintString,
			}
			err := githubRelease.Validate()
			assert.NoError(t, err)
			compiledConstraint, err := semver.NewConstraint(constraintString)
			assert.NoError(t, err)
			assert.Equal(t, compiledConstraint, githubRelease.compiledVersionConstraint)
		})

		t.Run("should set compiledVersionRegex", func(t *testing.T) {
			regexString := "v([a-zA-Z0-9]+)"
			githubRelease := &GithubRelease{
				Owner:        "test-owner",
				Repository:   "test-repo",
				Images:       []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				VersionRegex: regexString,
			}
			err := githubRelease.Validate()
			assert.NoError(t, err)
			assert.Equal(t, regexp.MustCompile(regexString), githubRelease.compiledVersionRegex)
		})
	})

	t.Run("processTagToVersion", func(t *testing.T) {
		type testCase struct {
			Message         string
			GithubRelease   *GithubRelease
			Tag             string
			ExpectedVersion string
			ExpectedError   string
		}
		testCases := []testCase{
			{
				Message: "should not return passed tag if constraint and regex are not defined",
				GithubRelease: &GithubRelease{
					Owner:      "test-owner",
					Repository: "test-repo",
					Images:     []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
				},
				Tag:             "v1.2.3",
				ExpectedVersion: "v1.2.3",
			},
			{
				Message: "should process tag according to regex capture group",
				GithubRelease: &GithubRelease{
					Owner:        "test-owner",
					Repository:   "test-repo",
					Images:       []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionRegex: "^v(.*)$",
				},
				Tag:             "v1.2.3",
				ExpectedVersion: "1.2.3",
			},
			{
				Message: "should return empty version if passed tag does not match regex",
				GithubRelease: &GithubRelease{
					Owner:        "test-owner",
					Repository:   "test-repo",
					Images:       []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionRegex: "v(asdf)",
				},
				Tag:             "v1.2.3",
				ExpectedVersion: "",
			},
			{
				Message: "should return version if passed tag satisfies version constraint",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionConstraint: ">=1.0.0",
				},
				Tag:             "v1.2.3",
				ExpectedVersion: "v1.2.3",
			},
			{
				Message: "should return empty version if passed tag does not satisfy version constraint",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionConstraint: "<1.0.0",
				},
				Tag:             "v1.2.3",
				ExpectedVersion: "",
			},
			{
				Message: "should return error if version constraint is specified and found version is not valid regex",
				GithubRelease: &GithubRelease{
					Owner:             "test-owner",
					Repository:        "test-repo",
					Images:            []AutoupdateImageRef{{SourceImage: "rancher/rancher"}},
					VersionConstraint: "<1.0.0",
					VersionRegex:      "^v(.*)$",
				},
				Tag:           "v1.2asdf",
				ExpectedError: "error parsing release version: invalid semantic version",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.GithubRelease.Validate()
				assert.NoError(t, err)
				version, err := testCase.GithubRelease.processTagToVersion(testCase.Tag)
				assert.Equal(t, testCase.ExpectedVersion, version)
				if testCase.ExpectedError != "" {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
