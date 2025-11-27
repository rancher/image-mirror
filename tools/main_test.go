package main

import (
	"errors"
	"testing"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/stretchr/testify/assert"
)

func createArtifact(t *testing.T, sourceArtifact string, tags []string, targetArtifactName string) *config.Artifact {
	t.Helper()
	img, err := config.NewArtifact(sourceArtifact, tags, targetArtifactName, nil, nil)
	assert.NoError(t, err)
	return img
}

func TestCheckNoTagsRemoved(t *testing.T) {
	type testCase struct {
		Name         string
		oldArtifacts []*config.Artifact
		NewArtifacts []*config.Artifact
		ExpectedErrs []error
	}

	testCases := []testCase{
		{
			Name: "no change",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag added",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag removed",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetArtifactName "mirrored-library-ubuntu")`),
			},
		},
		{
			Name: "multiple tags removed",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04", "24.04"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetArtifactName "mirrored-library-ubuntu")`),
				errors.New(`library/ubuntu:24.04 removed (TargetArtifactName "mirrored-library-ubuntu")`),
			},
		},
		{
			Name: "artifact removed",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
				createArtifact(t, "library/alpine", []string{"3.14"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/alpine:3.14 removed (TargetArtifactName "mirrored-library-alpine")`),
			},
		},
		{
			Name: "artifact added",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, ""),
				createArtifact(t, "library/alpine", []string{"3.14"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name:         "empty slices",
			oldArtifacts: []*config.Artifact{},
			NewArtifacts: []*config.Artifact{},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag removed from artifact with custom target name",
			oldArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04", "22.04"}, "my-ubuntu"),
			},
			NewArtifacts: []*config.Artifact{
				createArtifact(t, "library/ubuntu", []string{"20.04"}, "my-ubuntu"),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetArtifactName "my-ubuntu")`),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			var errs []error
			checkNoTagsRemoved(&errs, testCase.oldArtifacts, testCase.NewArtifacts)
			assert.Len(t, errs, len(testCase.ExpectedErrs))
			assert.ElementsMatch(t, testCase.ExpectedErrs, errs)
		})
	}
}
