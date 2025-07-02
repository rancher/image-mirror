package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should exclude repos with Target: false from sync entries", func(t *testing.T) {
			image1, err := NewImage("test-org/image1", []string{"v1.0.0"})
			assert.NoError(t, err)
			image2, err := NewImage("test-org/image2", []string{"v2.0.0"})
			assert.NoError(t, err)
			config := &Config{
				Images: []*Image{image1, image2},
				Repositories: []Repository{
					{
						BaseUrl:  "docker.io/target-repo",
						Target:   true,
						Username: "target-user",
						Password: "target-pass",
						Registry: "docker.io",
					},
					{
						BaseUrl:  "docker.io/non-target-repo",
						Target:   false,
						Username: "non-target-user",
						Password: "non-target-pass",
						Registry: "docker.io",
					},
				},
			}

			regsyncYaml, err := config.ToRegsyncConfig()
			assert.NoError(t, err)

			// Non-target repos should still be included in regsync.yaml, since
			// they may be the source of some images.
			assert.Len(t, regsyncYaml.Creds, 2)

			for _, regsyncYamlImage := range regsyncYaml.Sync {
				assert.True(t, strings.HasPrefix(regsyncYamlImage.Target, config.Repositories[0].BaseUrl))
			}
		})
	})
}
