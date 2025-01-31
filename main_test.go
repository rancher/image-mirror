package main

import (
	"fmt"
	"testing"

	"github.com/rancher/image-mirror/pkg/config"
	"github.com/rancher/image-mirror/pkg/regsync"
	"github.com/stretchr/testify/assert"
)

func TestGetRegsyncEntries(t *testing.T) {
	for _, targetImageName := range []string{"", "other-org-test-image"} {
		t.Run(fmt.Sprintf("should return correct regsync entries with TargetImageName set to %q", targetImageName), func(t *testing.T) {
			sourceOrg := "test-org"
			sourceImageName := "test-image"
			expectedTargetImageName := targetImageName
			if targetImageName == "" {
				expectedTargetImageName = "mirrored-" + sourceOrg + "-" + sourceImageName
			}
			image := config.Image{
				SourceImage:     sourceOrg + "/" + sourceImageName,
				TargetImageName: targetImageName,
				Tags: []string{
					"v1.2.3",
					"v2.3.4",
				},
			}
			repo := config.Repository{
				EnvVarPrefix: "TEST1",
				BaseUrl:      "docker.io/test1",
			}
			regsyncEntries, err := getRegsyncEntries(repo, image)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Len(t, regsyncEntries, 2)
			expectedEntries := []regsync.ConfigSync{
				{
					Source: "test-org/test-image:v1.2.3",
					Target: "docker.io/test1/" + expectedTargetImageName + ":v1.2.3",
					Type:   "image",
				},
				{
					Source: "test-org/test-image:v2.3.4",
					Target: "docker.io/test1/" + expectedTargetImageName + ":v2.3.4",
					Type:   "image",
				},
			}
			for _, expectedEntry := range expectedEntries {
				assert.Contains(t, regsyncEntries, expectedEntry)
			}
		})
	}
}
