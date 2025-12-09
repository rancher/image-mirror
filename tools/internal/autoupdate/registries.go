package autoupdate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type DockerHub struct {
	Namespace  string
	Repository string
}

func (d DockerHub) getArtifactTags() ([]string, error) {
	var allTags []string
	page := 1

	for {
		tags, hasNext, err := d.fetchPage(page)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)

		if !hasNext {
			break
		}
		page++
	}

	return allTags, nil
}

func (d DockerHub) fetchPage(page int) ([]string, bool, error) {
	params := url.Values{}
	params.Add("page", strconv.Itoa(page))
	params.Add("page_size", "100")
	reqUrl := fmt.Sprintf("https://registry.hub.docker.com/v2/namespaces/%s/repositories/%s/tags", d.Namespace, d.Repository)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	resp, err := doRequestWithRetries(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %w", err)
	}
	var data DockerHubResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, err
	}
	tags := make([]string, len(data.Results))
	for i, tag := range data.Results {
		tags[i] = tag.Name
	}
	return tags, data.Next != "", nil
}

type QuayIO struct {
	Namespace  string
	Repository string
}

func (q QuayIO) getArtifactTags() ([]string, error) {
	var allTags []string
	page := 1

	for {
		tags, hasNext, err := q.fetchPage(page)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)

		if !hasNext {
			break
		}
		page++
	}

	return allTags, nil
}

func (q QuayIO) fetchPage(page int) ([]string, bool, error) {
	params := url.Values{}
	params.Add("page", strconv.Itoa(page))
	params.Add("page_size", "100")
	reqUrl := fmt.Sprintf("https://quay.io/api/v1/repository/%s/%s/tag/", q.Namespace, q.Repository)
	req, err := http.NewRequest(http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.URL.RawQuery = params.Encode()
	resp, err := doRequestWithRetries(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %w", err)
	}
	var data QuayResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, err
	}
	tags := make([]string, len(data.Tags))
	for i, tag := range data.Tags {
		tags[i] = tag.Name
	}
	return tags, data.HasAdditional, nil
}

type SUSERegistry struct {
	Namespace  string
	Repository string
}

func (s SUSERegistry) getArtifactTags() ([]string, error) {
	token, err := s.getSuseAuthToken(s.Namespace, s.Repository)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://registry.suse.com/v2/%s/%s/tags/list", s.Namespace, s.Repository), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := doRequestWithRetries(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var data GenericTagsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data.Tags, nil
}

func (s SUSERegistry) getSuseAuthToken(namespace, repository string) (string, error) {
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

type GitHubRegistry struct {
	Namespace  string
	Repository string
}

func (g GitHubRegistry) getArtifactTags() ([]string, error) {
	githubToken := os.Getenv("GITHUB_TOKEN")
	token := base64.StdEncoding.EncodeToString([]byte(githubToken))
	var AllTags []string
	var nextUrl string
	for {
		tags, next, err := g.fetchTags(token, nextUrl)
		if err != nil {
			return nil, err
		}
		AllTags = append(AllTags, tags...)
		if next == "" {
			break
		}
		nextUrl = next
	}
	return AllTags, nil
}

func (g GitHubRegistry) fetchTags(token, url string) ([]string, string, error) {
	var registryUrl string
	if url != "" {
		registryUrl = url
	} else {
		registryUrl = fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", g.Namespace, g.Repository)
	}
	req, err := http.NewRequest(http.MethodGet, registryUrl, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := doRequestWithRetries(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	linkHeader := resp.Header.Get("Link")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}
	var data GenericTagsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, "", err
	}
	nextLink := parseLinkHeader(linkHeader)
	if nextLink == "" {
		return data.Tags, "", nil
	}
	return data.Tags, "https://ghcr.io" + nextLink, nil
}

type GoogleRegistry struct {
	Namespace  string
	Repository string
}

func (g GoogleRegistry) getArtifactTags() ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://gcr.io/v2/%s/%s/tags/list", g.Namespace, g.Repository), nil)
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
	var data GenericTagsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data.Tags, nil
}

type K8sRegistry struct {
	Namespace  string
	Repository string
}

func (k K8sRegistry) getArtifactTags() ([]string, error) {
	var registryUrl string
	if k.Namespace != "" {
		registryUrl = fmt.Sprintf("https://registry.k8s.io/v2/%s/%s/tags/list", k.Namespace, k.Repository)
	} else {
		registryUrl = fmt.Sprintf("https://registry.k8s.io/v2/%s/tags/list", k.Repository)
	}

	req, err := http.NewRequest(http.MethodGet, registryUrl, nil)
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
	var data GenericTagsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data.Tags, nil
}

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
