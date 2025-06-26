package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmLatest(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message       string
			HelmLatest    *HelmLatest
			ExpectedError string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid HelmLatest",
				HelmLatest: &HelmLatest{
					HelmRepo: "https://helm.cilium.io",
					Charts: map[string]map[string]Environment{
						"cilium": {
							"default": {},
						},
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return error for empty HelmRepo",
				HelmLatest: &HelmLatest{
					Charts: map[string]map[string]Environment{
						"cilium": {
							"default": {},
						},
					},
				},
				ExpectedError: "must specify HelmRepo",
			},
			{
				Message: "should return error for nil Charts",
				HelmLatest: &HelmLatest{
					HelmRepo: "https://helm.cilium.io",
				},
				ExpectedError: "must specify at least one chart in Charts",
			},
			{
				Message: "should return error for empty Charts",
				HelmLatest: &HelmLatest{
					HelmRepo: "https://helm.cilium.io",
					Charts:   map[string]map[string]Environment{},
				},
				ExpectedError: "must specify at least one chart in Charts",
			},
			{
				Message: "should return error for chart with no environments",
				HelmLatest: &HelmLatest{
					HelmRepo: "https://helm.cilium.io",
					Charts: map[string]map[string]Environment{
						"cilium": {},
					},
				},
				ExpectedError: `chart "cilium" must have at least one environment`,
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.HelmLatest.Validate()
				if testCase.ExpectedError == "" {
					assert.Nil(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
