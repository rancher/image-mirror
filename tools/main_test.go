package main

import (
	"errors"
	"testing"

	"github.com/rancher/image-mirror/internal/config"

	"github.com/stretchr/testify/assert"
)

func createImage(t *testing.T, sourceImage string, tags []string, targetImageName string) *config.Image {
	t.Helper()
	img, err := config.NewImage(sourceImage, tags, targetImageName, nil, nil)
	assert.NoError(t, err)
	return img
}

func TestCheckNoTagsRemoved(t *testing.T) {
	type testCase struct {
		Name         string
		oldImages    []*config.Image
		NewImages    []*config.Image
		ExpectedErrs []error
	}

	testCases := []testCase{
		{
			Name: "no change",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag added",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag removed",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetImageName "mirrored-library-ubuntu")`),
			},
		},
		{
			Name: "multiple tags removed",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04", "24.04"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetImageName "mirrored-library-ubuntu")`),
				errors.New(`library/ubuntu:24.04 removed (TargetImageName "mirrored-library-ubuntu")`),
			},
		},
		{
			Name: "image removed",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
				createImage(t, "library/alpine", []string{"3.14"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			ExpectedErrs: []error{
				errors.New(`library/alpine:3.14 removed (TargetImageName "mirrored-library-alpine")`),
			},
		},
		{
			Name: "image added",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, ""),
				createImage(t, "library/alpine", []string{"3.14"}, ""),
			},
			ExpectedErrs: []error{},
		},
		{
			Name:         "empty slices",
			oldImages:    []*config.Image{},
			NewImages:    []*config.Image{},
			ExpectedErrs: []error{},
		},
		{
			Name: "tag removed from image with custom target name",
			oldImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04", "22.04"}, "my-ubuntu"),
			},
			NewImages: []*config.Image{
				createImage(t, "library/ubuntu", []string{"20.04"}, "my-ubuntu"),
			},
			ExpectedErrs: []error{
				errors.New(`library/ubuntu:22.04 removed (TargetImageName "my-ubuntu")`),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			var errs []error
			checkNoTagsRemoved(&errs, testCase.oldImages, testCase.NewImages)
			assert.Len(t, errs, len(testCase.ExpectedErrs))
			assert.ElementsMatch(t, testCase.ExpectedErrs, errs)
		})
	}
}
