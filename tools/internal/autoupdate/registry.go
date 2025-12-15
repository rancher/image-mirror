package autoupdate

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/artifact-mirror/internal/config"
)

type Registry struct {
	Artifacts     []AutoupdateArtifactRef
	Latest        bool   `json:",omitempty"`
	VersionFilter string `json:",omitempty"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

type ArtifactRegistry interface {
	getArtifactTags() ([]string, error)
}

func (r *Registry) GetUpdateArtifacts() ([]*config.Artifact, error) {
	allTags, err := r.getArtifactTags()
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact tags: %w", err)
	}

	if len(allTags) == 0 {
		return nil, fmt.Errorf("no artifact tags found")
	}

	var filteredTags []string
	if r.VersionFilter != "" {
		versionFilter := regexp.MustCompile(r.VersionFilter)

		for _, tag := range allTags {
			if !versionFilter.MatchString(tag) {
				continue
			}
			filteredTags = append(filteredTags, tag)
		}

		if len(filteredTags) == 0 {
			return nil, errors.New("no tags found matching version filter")
		}

	} else {
		filteredTags = allTags
	}

	if r.Latest {
		vs := make([]*semver.Version, len(filteredTags))
		for i, r := range filteredTags {
			v, err := semver.NewVersion(r)
			if err != nil {
				return nil, fmt.Errorf("error parsing version: %s", err)
			}
			vs[i] = v
		}
		sort.Sort(semver.Collection(vs))
		filteredTags = []string{vs[len(vs)-1].String()} // Use the latest version
	}

	artifacts := make([]*config.Artifact, 0, len(r.Artifacts))
	for _, sourceArtifact := range r.Artifacts {
		artifact, err := config.NewArtifact(sourceArtifact.SourceArtifact, filteredTags, sourceArtifact.TargetArtifactName, nil, nil)
		if err != nil {
			return nil, err
		}
		artifact.SetTargetArtifactName(sourceArtifact.TargetArtifactName)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func (r *Registry) Validate() error {
	if len(r.Artifacts) == 0 {
		return errors.New("must specify at least one artifact")
	}
	if r.VersionFilter != "" {
		if _, err := regexp.Compile(r.VersionFilter); err != nil {
			return errors.New("invalid version filter regex: " + err.Error())
		}
	}
	return nil
}

func (r *Registry) getArtifactTags() ([]string, error) {
	registry, err := r.getRegistryInformationFromArtifact()
	if err != nil {
		return nil, fmt.Errorf("failed to get registry information from artifact: %w", err)
	}
	return registry.getArtifactTags()
}

func (r *Registry) getRegistryInformationFromArtifact() (ArtifactRegistry, error) {
	artifact := r.Artifacts[0].SourceArtifact
	splittedArtifact := strings.Split(artifact, "/")
	if len(splittedArtifact) < 2 {
		return nil, fmt.Errorf("invalid artifact format: %s", artifact)
	}
	var registry, namespace, repository string
	// Case 1: Handle default Docker Hub artifacts like "flannel/flannel"
	if len(splittedArtifact) == 2 && !strings.Contains(splittedArtifact[0], ".") {
		registry = "dockerhub"
		namespace = splittedArtifact[0]
		repository = splittedArtifact[1]
	} else if len(splittedArtifact) == 2 && strings.Contains(splittedArtifact[0], ".") {
		// Case 2: Handle artifacts with a registry but no namespace like "k8s.gcr.io/pause"
		registry = splittedArtifact[0]
		namespace = ""
		repository = splittedArtifact[1]
	} else {
		// Default Case: Handle standard 3-part artifacts like "quay.io/skopeo/stable"
		// and handle artifacts with long paths like "gcr.io/cloud-provider-vsphere/csi/release/syncer"
		registry = splittedArtifact[0]
		namespace = splittedArtifact[1]
		repository = strings.Join(splittedArtifact[2:], "/")
	}
	switch registry {
	case "dockerhub":
		return &DockerHub{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	case "quay.io":
		return &QuayIO{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	case "registry.k8s.io":
		return &K8sRegistry{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	case "registry.suse.com":
		return &SUSERegistry{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	case "ghcr.io":
		return &GitHubRegistry{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	case "gcr.io":
		return &GoogleRegistry{
			Namespace:  namespace,
			Repository: repository,
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized registry: %s", registry)
	}
}

func doRequestWithRetries(req *http.Request) (*http.Response, error) {
	maxRetries := 10
	backoffFactor := time.Second
	retryableStatuses := map[int]bool{
		http.StatusInternalServerError: true, // 500
		http.StatusBadGateway:          true, // 502
		http.StatusServiceUnavailable:  true, // 503
		http.StatusGatewayTimeout:      true, // 504
	}

	var resp *http.Response
	var err error
	var tries int

	for {
		resp, err = httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		tries++
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if !retryableStatuses[resp.StatusCode] {
			b, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("request to %s failed with status %s", req.URL.String(), resp.Status)
			}

			resp.Body.Close()
			return nil, fmt.Errorf("request to %s failed with status %s and body %s", req.URL.String(), resp.Status, string(b))
		}

		if resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			err := resp.Body.Close()
			if err != nil {
				return nil, err
			}
		}

		if tries == maxRetries {
			break
		}

		sleepDuration := backoffFactor * time.Duration(math.Pow(2, float64(tries)))
		time.Sleep(sleepDuration)
	}

	return nil, fmt.Errorf("request failed after %d retries", maxRetries)
}

// parseLinkHeader extracts the next URL from the Link header for pagination.
// Example: </v2/epinio/epinio-server/tags/list?last=v1.10.0-rc1-5-g0d7c9121&n=200>; rel="next"
func parseLinkHeader(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	link := strings.TrimPrefix(linkHeader, "<")
	link = strings.TrimSuffix(link, `>; rel="next"`)
	return link
}
