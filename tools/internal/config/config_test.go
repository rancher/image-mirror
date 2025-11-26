package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should always include Registries in regsync config even if DefaultTarget field is false", func(t *testing.T) {
			// Non-target registries should still be included in regsync.yaml, since
			// they may be the source of some images.
			config := &Config{
				Images: []*Image{},
				Registries: []Registry{
					{
						BaseUrl:       "docker.io/target-registry",
						DefaultTarget: true,
						Username:      "target-user",
						Password:      "target-pass",
						Registry:      "docker.io",
					},
					{
						BaseUrl:       "docker.io/non-target-registry",
						DefaultTarget: false,
						Username:      "non-target-user",
						Password:      "non-target-pass",
						Registry:      "docker.io",
					},
				},
			}

			regsyncYaml, err := config.ToRegsyncConfig()
			assert.NoError(t, err)

			assert.Len(t, regsyncYaml.Creds, 2)
		})
	})
}
