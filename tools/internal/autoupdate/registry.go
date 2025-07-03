package autoupdate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/image-mirror/internal/config"
)

type Registry struct {
	Images        []AutoupdateImageRef
	Latest        bool   `json:",omitempty"`
	VersionFilter string `json:",omitempty"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// DockerHubResponse matches the structure of the Docker Hub API response
type DockerHubResponse struct {
	Next    string `json:"next"`
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}

// QuayResponse matches the structure of the Quay.io API response
type QuayResponse struct {
	HasAdditional bool `json:"has_additional"`
	Tags          []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

type SuseTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type GenericTagsResponse struct {
	Tags []string `json:"tags"`
}

func (r *Registry) GetUpdateImages() ([]*config.Image, error) {
	githubToken := os.Getenv("GITHUB_TOKEN")
	allTags, err := r.getImageTags(githubToken, "", 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get image tags: %w", err)
	}

	if len(allTags) == 0 {
		return nil, fmt.Errorf("no image tags found")
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

	images := make([]*config.Image, 0, len(r.Images))
	for _, sourceImage := range r.Images {
		image, err := config.NewImage(sourceImage.SourceImage, filteredTags, sourceImage.TargetImageName, nil)
		if err != nil {
			return nil, err
		}
		image.SetTargetImageName(sourceImage.TargetImageName)
		images = append(images, image)
	}
	return images, nil
}

func (r *Registry) Validate() error {
	if len(r.Images) == 0 {
		return errors.New("must specify at least one image")
	}
	if r.VersionFilter != "" {
		if _, err := regexp.Compile(r.VersionFilter); err != nil {
			return errors.New("invalid version filter regex: " + err.Error())
		}
	}
	return nil
}

func (r *Registry) getImageTags(githubToken, nextURL string, page int) ([]string, error) {
	registry, namespace, repository := r.getRegistryInformationFromImage()

	req, err := buildRequest(registry, namespace, repository, fmt.Sprintf("%d", page), nextURL, githubToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := doRequestWithRetries(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return r.processRegistryResponse(body, resp.Header.Get("Link"), githubToken, registry, namespace, repository, page)
}

func buildRequest(registry, namespace, repository, page, nextUrl, githubToken string) (*http.Request, error) {
	params := url.Values{}
	var registryUrl, token string
	switch registry {
	case "dockerhub":
		params.Add("page", page)
		params.Add("page_size", "100")
		registryUrl = fmt.Sprintf("https://registry.hub.docker.com/v2/namespaces/%s/repositories/%s/tags", namespace, repository)
	case "quay.io":
		params.Add("page", page)
		params.Add("page_size", "100")
		registryUrl = fmt.Sprintf("https://quay.io/api/v1/repository/%s/%s/tag/", namespace, repository)
	case "registry.k8s.io":
		if namespace != "" {
			registryUrl = fmt.Sprintf("https://registry.k8s.io/v2/%s/%s/tags/list", namespace, repository)
		} else {
			registryUrl = fmt.Sprintf("https://registry.k8s.io/v2/%s/tags/list", repository)
		}
	case "registry.suse.com":
		var err error
		token, err = getSuseAuthToken(namespace, repository)
		if err != nil {
			return nil, err
		}
		registryUrl = fmt.Sprintf("https://registry.suse.com/v2/%s/%s/tags/list", namespace, repository)
	case "ghcr.io":
		token = base64.StdEncoding.EncodeToString([]byte(githubToken))
		if nextUrl != "" {
			registryUrl = nextUrl
		} else {
			registryUrl = fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", namespace, repository)
		}
	case "gcr.io":
		registryUrl = fmt.Sprintf("https://gcr.io/v2/%s/%s/tags/list", namespace, repository)
	default:
		return nil, fmt.Errorf("unrecognized registry: %s", registry)
	}
	req, err := http.NewRequest(http.MethodGet, registryUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.URL.RawQuery = params.Encode()
	return req, nil
}

func (r *Registry) getRegistryInformationFromImage() (registry string, namespace string, repository string) {
	image := r.Images[0].SourceImage
	splittedImage := strings.Split(image, "/")

	// Case 1: Handle default Docker Hub images like "flannel/flannel"
	if len(splittedImage) == 2 && !strings.Contains(splittedImage[0], ".") {
		return "dockerhub", splittedImage[0], splittedImage[1]
	}

	// Case 2: Handle images with a registry but no namespace like "k8s.gcr.io/pause"
	if len(splittedImage) == 2 && strings.Contains(splittedImage[0], ".") {
		return splittedImage[0], "", splittedImage[1]
	}

	// Case 3: Handle images with long paths like "gcr.io/cloud-provider-vsphere/csi/release/syncer"
	if len(splittedImage) > 3 && strings.Contains(splittedImage[0], ".") {
		return splittedImage[0], splittedImage[1], strings.Join(splittedImage[2:], "/")
	}

	// Default Case: Handle standard 3-part images like "quay.io/skopeo/stable"
	return splittedImage[0], splittedImage[1], splittedImage[2]
}

func getSuseAuthToken(namespace, repository string) (string, error) {
	tokenURL := "https://registry.suse.com/auth?service=SUSE+Linux+Docker+Registry"
	req, _ := http.NewRequest("GET", tokenURL, nil)
	params := req.URL.Query()
	params.Add("scope", fmt.Sprintf("repository:%s/%s:pull", namespace, repository))
	req.URL.RawQuery = params.Encode()

	resp, err := doRequestWithRetries(req)
	if err != nil {
		return "", fmt.Errorf("suse token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenData SuseTokenResponse
	if err := json.Unmarshal(body, &tokenData); err != nil {
		return "", fmt.Errorf("failed to unmarshal suse token: %w", err)
	}
	return tokenData.AccessToken, nil
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

func (r *Registry) processRegistryResponse(body []byte, linkHeader, githubToken, registry, namespace, pkg string, page int) ([]string, error) {
	switch registry {
	case "dockerhub":
		var data DockerHubResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		tags := make([]string, len(data.Results))
		for i, tag := range data.Results {
			tags[i] = tag.Name
		}
		if data.Next == "" {
			return tags, nil
		}
		nextTags, err := r.getImageTags(githubToken, "", page+1)
		if err != nil {
			return nil, err
		}
		return append(tags, nextTags...), nil

	case "quay.io":
		var data QuayResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		tags := make([]string, len(data.Tags))
		for i, tag := range data.Tags {
			tags[i] = tag.Name
		}
		if !data.HasAdditional {
			return tags, nil
		}
		nextTags, err := r.getImageTags(githubToken, "", page+1)
		if err != nil {
			return nil, err
		}
		return append(tags, nextTags...), nil

	case "registry.k8s.io", "registry.suse.com", "gcr.io":
		var data GenericTagsResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		return data.Tags, nil

	case "ghcr.io":
		var data GenericTagsResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		nextLink := parseLinkHeader(linkHeader)
		if nextLink == "" {
			return data.Tags, nil
		}
		nextURL := "https://ghcr.io" + nextLink
		nextTags, err := r.getImageTags(githubToken, nextURL, page)
		if err != nil {
			return nil, err
		}
		return append(data.Tags, nextTags...), nil
	}
	return nil, fmt.Errorf("unrecognized registry for processing: %s", registry)
}
