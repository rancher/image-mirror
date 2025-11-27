package config

import (
	"strings"
	"testing"

	"github.com/rancher/image-mirror/internal/regsync"
	"github.com/stretchr/testify/assert"
)

func TestArtifact(t *testing.T) {
	t.Run("ToRegsyncConfig", func(t *testing.T) {
		t.Run("should exclude artifacts with same source and target", func(t *testing.T) {
			type TestCase struct {
				Name            string
				ArtifactRef     string
				BaseUrl         string
				ExpectedPresent bool
			}
			testCases := []TestCase{
				{
					Name:            "dockerhub artifact with docker.io prefix and dockerhub base URL",
					ArtifactRef:     "docker.io/test-org/test-artifact",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: false,
				},
				{
					Name:            "dockerhub artifact with docker.io prefix and non-dockerhub base URL",
					ArtifactRef:     "docker.io/test-org/test-artifact",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "dockerhub artifact without docker.io prefix and with dockerhub base URL",
					ArtifactRef:     "test-org/test-artifact",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: false,
				},
				{
					Name:            "dockerhub artifact without docker.io prefix and with non-dockerhub base URL",
					ArtifactRef:     "test-org/test-artifact",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "non-dockerhub artifact with dockerhub base URL",
					ArtifactRef:     "some.other.registry/test-org/test-artifact",
					BaseUrl:         "docker.io/test-org",
					ExpectedPresent: true,
				},
				{
					Name:            "non-dockerhub artifact with non-dockerhub base URL",
					ArtifactRef:     "some.other.registry/test-org/test-artifact",
					BaseUrl:         "some.other.registry/test-org",
					ExpectedPresent: false,
				},
			}
			for _, testCase := range testCases {
				t.Run(testCase.Name, func(t *testing.T) {
					tag := "v1.0.0"
					artifact, err := NewArtifact(testCase.ArtifactRef, []string{tag}, "test-artifact", nil, nil)
					assert.NoError(t, err)
					repositories := []Repository{
						{
							BaseUrl:       testCase.BaseUrl,
							DefaultTarget: true,
						},
					}

					configEntries, err := artifact.ToRegsyncArtifacts(repositories)
					assert.NoError(t, err)

					if testCase.ExpectedPresent {
						assert.Len(t, configEntries, 1)
						assert.Equal(t, artifact.SourceArtifact+":"+tag, configEntries[0].Source)
						assert.Equal(t, testCase.BaseUrl+"/"+artifact.TargetArtifactName()+":"+tag, configEntries[0].Target)
					} else {
						assert.Len(t, configEntries, 0)
					}
				})
			}
		})

		t.Run("should only target repos with DefaultTarget set to true when TargetRepositories is not specified", func(t *testing.T) {
			artifact, err := NewArtifact("test-org/artifact", []string{"v1.0.0"}, "", nil, nil)
			assert.NoError(t, err)
			repositories := []Repository{
				{
					BaseUrl:       "docker.io/target-repo",
					DefaultTarget: true,
				},
				{
					BaseUrl:       "docker.io/non-target-repo",
					DefaultTarget: false,
				},
			}

			configEntries, err := artifact.ToRegsyncArtifacts(repositories)
			assert.NoError(t, err)

			assert.Len(t, configEntries, 1)
			assert.True(t, strings.HasPrefix(configEntries[0].Target, repositories[0].BaseUrl))
		})

		t.Run("should target only repositories specified by TargetRepositories, including ones with DefaultTarget: false", func(t *testing.T) {
			repositories := []Repository{
				{
					BaseUrl:       "site0.com/registry",
					DefaultTarget: true,
				},
				{
					BaseUrl:       "site1.com/registry",
					DefaultTarget: false,
				},
				{
					BaseUrl:       "site2.com/registry",
					DefaultTarget: true,
				},
				{
					BaseUrl:       "site3.com/registry",
					DefaultTarget: false,
				},
			}
			tags := []string{"v1.0.0"}
			targetRepositories := []string{"site0.com/registry", "site1.com/registry"}
			artifact, err := NewArtifact("test-org/artifact", tags, "", nil, targetRepositories)
			assert.NoError(t, err)

			configEntries, err := artifact.ToRegsyncArtifacts(repositories)
			assert.NoError(t, err)

			assert.Len(t, configEntries, len(tags)*len(targetRepositories))
			for _, configEntry := range configEntries {
				matches0 := strings.HasPrefix(configEntry.Target, targetRepositories[0])
				matches1 := strings.HasPrefix(configEntry.Target, targetRepositories[1])
				assert.True(t, matches0 || matches1)
			}
		})
	})

	t.Run("ToRegsyncArtifactsForSingleRepository", func(t *testing.T) {
		type TestCase struct {
			Name                        string
			SpecifiedTargetArtifactName string
			DoNotMirror                 any
			ExpectedEntries             []regsync.ConfigSync
		}
		for _, testCase := range []TestCase{
			{
				Name:                        "should use default artifact name when TargetArtifactName is not set",
				SpecifiedTargetArtifactName: "",
				DoNotMirror:                 nil,
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-artifact:v1.2.3",
						Target: "docker.io/test1/mirrored-test-org-test-artifact:v1.2.3",
						Type:   "image",
					},
					{
						Source: "test-org/test-artifact:v2.3.4",
						Target: "docker.io/test1/mirrored-test-org-test-artifact:v2.3.4",
						Type:   "image",
					},
				},
			},
			{
				Name:                        "should use TargetArtifactName when it is set",
				SpecifiedTargetArtifactName: "other-org-test-artifact",
				DoNotMirror:                 nil,
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-artifact:v1.2.3",
						Target: "docker.io/test1/other-org-test-artifact:v1.2.3",
						Type:   "image",
					},
					{
						Source: "test-org/test-artifact:v2.3.4",
						Target: "docker.io/test1/other-org-test-artifact:v2.3.4",
						Type:   "image",
					},
				},
			},
			{
				Name:                        "should not return any entries if DoNotMirror is true",
				SpecifiedTargetArtifactName: "",
				DoNotMirror:                 true,
				ExpectedEntries:             []regsync.ConfigSync{},
			},
			{
				Name:                        "should only return unspecified tags if DoNotMirror specifies tags",
				SpecifiedTargetArtifactName: "",
				DoNotMirror:                 []any{"v2.3.4"},
				ExpectedEntries: []regsync.ConfigSync{
					{
						Source: "test-org/test-artifact:v1.2.3",
						Target: "docker.io/test1/mirrored-test-org-test-artifact:v1.2.3",
						Type:   "image",
					},
				},
			},
		} {
			t.Run(testCase.Name, func(t *testing.T) {
				inputArtifact, err := NewArtifact("test-org/test-artifact", []string{"v1.2.3", "v2.3.4"}, testCase.SpecifiedTargetArtifactName, testCase.DoNotMirror, nil)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				inputRepository := Repository{
					BaseUrl: "docker.io/test1",
				}
				regsyncEntries, err := inputArtifact.ToRegsyncArtifactsForSingleRepository(inputRepository)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				assert.Len(t, regsyncEntries, len(testCase.ExpectedEntries))
				for _, expectedEntry := range testCase.ExpectedEntries {
					assert.Contains(t, regsyncEntries, expectedEntry)
				}
			})
		}
	})

	t.Run("setDefaults", func(t *testing.T) {
		t.Run("should return error for invalid DoNotMirror type", func(t *testing.T) {
			artifact := Artifact{
				SourceArtifact: "test/test",
				Tags:           []string{"tag1"},
				DoNotMirror:    1234,
			}
			err := artifact.setDefaults()
			assert.ErrorContains(t, err, "DoNotMirror must be nil, bool, or []any")
		})

		t.Run("should return error when DoNotMirror has invalid element type", func(t *testing.T) {
			artifact := Artifact{
				SourceArtifact: "test/test",
				Tags:           []string{"tag1"},
				DoNotMirror:    []any{"asdf", 1234, "qwer123"},
			}
			err := artifact.setDefaults()
			assert.Errorf(t, err, "failed to cast %v to string", 1234)
		})

		t.Run("should return error when DoNotMirror has a duplicated element", func(t *testing.T) {
			artifact := Artifact{
				SourceArtifact: "test/test",
				Tags:           []string{"tag1"},
				DoNotMirror:    []any{"asdf", "qwer", "asdf"},
			}
			err := artifact.setDefaults()
			assert.Error(t, err, "DoNotMirror entry asdf is duplicated")
		})

		t.Run("should return nil for valid DoNotMirror type", func(t *testing.T) {
			artifact := Artifact{
				SourceArtifact: "test/test",
				Tags:           []string{"tag1"},
			}
			doNotMirrorValues := []any{nil, true, []any{"tag1", "tag2"}}
			for _, doNotMirrorValue := range doNotMirrorValues {
				artifact.DoNotMirror = doNotMirrorValue
				err := artifact.setDefaults()
				assert.NoError(t, err)
			}
		})
	})

	t.Run("DeepCopy", func(t *testing.T) {
		t.Run("should copy all fields", func(t *testing.T) {
			original, err := NewArtifact("test-org/test-artifact", []string{"v1.0.0", "v2.0.0"}, "custom-image-name", []any{"v1.0.0"}, nil)
			assert.NoError(t, err)

			copy := original.DeepCopy()

			assert.Equal(t, original.DoNotMirror, copy.DoNotMirror)
			assert.Equal(t, original.SourceArtifact, copy.SourceArtifact)
			assert.Equal(t, original.defaultTargetArtifactName, copy.defaultTargetArtifactName)
			assert.Equal(t, original.SpecifiedTargetArtifactName, copy.SpecifiedTargetArtifactName)
			assert.Equal(t, original.Tags, copy.Tags)
			assert.Equal(t, original.excludeAllTags, copy.excludeAllTags)
			assert.Equal(t, original.excludedTags, copy.excludedTags)
			assert.Equal(t, original.TargetRepositories, copy.TargetRepositories)
		})
	})
}
