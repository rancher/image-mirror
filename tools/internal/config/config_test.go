package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should always include Repositories in regsync config even if DefaultTarget field is false", func(t *testing.T) {
			// Non-target repos should still be included in regsync.yaml, since
			// they may be the source of some artifacts.
			config := &Config{
				Artifacts: []*Artifact{},
				Repositories: []Repository{
					{
						BaseUrl:       "docker.io/target-repo",
						DefaultTarget: true,
						Username:      "target-user",
						Password:      "target-pass",
						Registry:      "docker.io",
					},
					{
						BaseUrl:       "docker.io/non-target-repo",
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
