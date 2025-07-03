package autoupdate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/rancher/image-mirror/internal/config"
	"gopkg.in/yaml.v3"
)

const helmRepoName = "image-mirror-tools-temp"

// An Environment is a set of configuration we would like to apply to a
// chart when templating it out and searching the result for images.
type Environment []string

// ToHelmTemplateArgs converts the environment to helm template --set arguments
func (e Environment) ToHelmTemplateArgs() []string {
	args := make([]string, 0, len(e)*2)
	for _, value := range e {
		args = append(args, "--set", value)
	}
	return args
}

// HelmLatest retrieves image references by templating the latest version of the
// configured helm chart, and recursively finding all fields with an "image" key.
// This is not a perfectly reliable way of finding images that a chart uses, and
// does not attempt to be. However, it is probably good enough for simple charts.
type HelmLatest struct {
	// Images tells the autoupdate code which entry in config.yaml to add the
	// update images to.
	Images []AutoupdateImageRef `json:",omitempty"`
	// HelmRepo is the URL of the Helm chart repository.
	HelmRepo string
	// Charts is a map of chart names to the environments under which we want to
	// search them for images.
	Charts map[string]map[string]Environment
	// ImageDenylist is a list of images to exclude from the result.
	ImageDenylist []string `json:",omitempty"`
}

// GetUpdateImages templates the helm chart and extracts all image references
func (hl *HelmLatest) GetUpdateImages() ([]*config.Image, error) {
	if _, err := exec.LookPath("helm"); err != nil {
		return nil, fmt.Errorf("helm command not found: %w", err)
	}

	imageMap := make(map[string][]string)

	addRepoCmd := exec.Command("helm", "repo", "add", helmRepoName, hl.HelmRepo)
	if err := addRepoCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to add helm repository: %w", err)
	}
	defer exec.Command("helm", "repo", "remove", helmRepoName).Run()

	updateCmd := exec.Command("helm", "repo", "update")
	if err := updateCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to update helm repositories: %w", err)
	}

	for chartName, environmentMap := range hl.Charts {
		for environmentName, environment := range environmentMap {
			args := []string{
				"template", chartName + "-" + environmentName,
				helmRepoName + "/" + chartName,
			}
			args = append(args, environment.ToHelmTemplateArgs()...)

			templateCmd := exec.Command("helm", args...)
			templateOutput := &bytes.Buffer{}
			templateCmd.Stdout = templateOutput
			if err := templateCmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to template chart %s for env %s: %w", chartName, environmentName, err)
			}

			decoder := yaml.NewDecoder(templateOutput)
			for {
				var templateData any
				if err := decoder.Decode(&templateData); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					return nil, fmt.Errorf("failed to parse template command output as yaml: %w", err)
				}
				if err := hl.extractImagesFromYaml(templateData, imageMap); err != nil {
					return nil, fmt.Errorf("failed to extract images from chart %s env %s: %w", chartName, environmentName, err)
				}
			}
		}
	}

	// Convert the map to a slice of config.Image objects
	images := make([]*config.Image, 0, len(imageMap))
	for sourceImage, tags := range imageMap {
		foundTargetImageName := ""
		for _, autoupdateImageRef := range hl.Images {
			if sourceImage == autoupdateImageRef.SourceImage {
				foundTargetImageName = autoupdateImageRef.TargetImageName
				break
			}
		}
		if foundTargetImageName == "" {
			return nil, fmt.Errorf("found image %s but it is not present in Images", sourceImage)
		}

		image, err := config.NewImage(sourceImage, tags, foundTargetImageName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create image: %w", err)
		}

		images = append(images, image)
	}

	// Filter out denied images
	filteredImages := make([]*config.Image, 0, len(images))
	for _, image := range images {
		if !slices.Contains(hl.ImageDenylist, image.SourceImage) {
			filteredImages = append(filteredImages, image)
		}
	}

	return filteredImages, nil
}

func (hl *HelmLatest) extractImagesFromYaml(data any, imageMap map[string][]string) error {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			if key == "image" {
				stringValue, ok := value.(string)
				if ok {
					repository, tag, err := hl.parseImageRef(stringValue)
					if err != nil {
						return fmt.Errorf("failed to parse %q as image ref: %w", stringValue, err)
					}
					existingTags, present := imageMap[repository]
					if !present {
						imageMap[repository] = []string{tag}
					} else if !slices.Contains(existingTags, tag) {
						imageMap[repository] = append(imageMap[repository], tag)
					}
				}
			} else {
				if err := hl.extractImagesFromYaml(value, imageMap); err != nil {
					return err
				}
			}
		}
	case []any:
		for _, item := range v {
			if err := hl.extractImagesFromYaml(item, imageMap); err != nil {
				return err
			}
		}
	}

	return nil
}

func (hl *HelmLatest) parseImageRef(rawString string) (string, string, error) {
	withoutDigest := strings.Split(rawString, "@")[0]
	parts := strings.Split(withoutDigest, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf(`failed to split %q into two parts on ":"`, withoutDigest)
	}
	repository := parts[0]
	tag := parts[1]
	repositoryWithoutDocker := strings.TrimPrefix(repository, "docker.io/")
	return repositoryWithoutDocker, tag, nil
}

func (hl *HelmLatest) Validate() error {
	if hl.HelmRepo == "" {
		return errors.New("must specify HelmRepo")
	}

	if len(hl.Charts) == 0 {
		return errors.New("must specify at least one chart in Charts")
	}

	for chartName, environments := range hl.Charts {
		if len(environments) == 0 {
			return fmt.Errorf("chart %q must have at least one environment", chartName)
		}
	}

	return nil
}
