package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rancher/image-mirror/pkg/config"
	"github.com/rancher/image-mirror/pkg/regsync"
	"github.com/urfave/cli/v2"
)

const configYamlPath = "config.yaml"
const regsyncYamlPath = "regsync.yaml"

func main() {
	log.SetFlags(0)

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:   "generate-regsync",
				Usage:  "Generate regsync.yaml",
				Action: generateRegsyncYaml,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateRegsyncYaml(ctx *cli.Context) error {
	cfg, err := config.Parse(configYamlPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", configYamlPath, err)
	}

	regsyncYaml := regsync.Config{
		Creds: make([]regsync.ConfigCred, 0, len(cfg.Repositories)),
		Sync:  make([]regsync.ConfigSync, 0),
	}
	for _, targetRepository := range cfg.Repositories {
		credEntry := regsync.ConfigCred{
			Registry: fmt.Sprintf(`{{ env "%s_ENDPOINT" }}`, targetRepository.EnvVarPrefix),
			User:     fmt.Sprintf(`{{ env "%s_USERNAME" }}`, targetRepository.EnvVarPrefix),
			Pass:     fmt.Sprintf(`{{ env "%s_PASSWORD" }}`, targetRepository.EnvVarPrefix),
		}
		regsyncYaml.Creds = append(regsyncYaml.Creds, credEntry)
	}
	for _, image := range cfg.Images {
		for _, repo := range cfg.Repositories {
			if !repo.Target {
				continue
			}
			syncEntries, err := getRegsyncEntries(repo, image)
			if err != nil {
				return fmt.Errorf("failed to convert Image with SourceImage %q: %w", image.SourceImage, err)
			}
			regsyncYaml.Sync = append(regsyncYaml.Sync, syncEntries...)
		}
	}

	if err := regsync.WriteConfig(regsyncYamlPath, regsyncYaml); err != nil {
		return fmt.Errorf("failed to write %s: %w", regsyncYamlPath, err)
	}

	return nil
}

func getRegsyncEntries(repo config.Repository, image config.Image) ([]regsync.ConfigSync, error) {
	targetImageName := image.TargetImageName
	if targetImageName == "" {
		parts := strings.Split(image.SourceImage, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("source image split into %d parts (>=2 parts expected)", len(parts))
		}
		repoName := parts[len(parts)-2]
		imageName := parts[len(parts)-1]
		targetImageName = "mirrored-" + repoName + "-" + imageName
	}

	entries := make([]regsync.ConfigSync, 0, len(image.Tags))
	for _, tag := range image.Tags {
		sourceImage := image.SourceImage + ":" + tag
		targetImage := repo.BaseUrl + "/" + targetImageName + ":" + tag
		entry := regsync.ConfigSync{
			Source: sourceImage,
			Target: targetImage,
			Type:   "image",
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
