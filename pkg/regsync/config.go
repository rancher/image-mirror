// https://github.com/regclient/regclient/blob/main/cmd/regsync/config.go
// is a useful reference for this. Ideally we would reuse this code,
// but there are two problems with this:
//  1. It isn't designed for writing concise config files. None of the
//     type definitions have omitempty defined in the json struct tags,
//     so either the output would be very verbose, or we would have to
//     figure out a way of automatically modifying the types.
//  2. The code is defined in a main package, which means that we cannot
//     import it. We could use code generation to download it, but that
//     still leaves problem number 1.
package regsync

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// Config represents a regsync config file.
type Config struct {
	Version int          `json:"version,omitempty"`
	Creds   []ConfigCred `json:"creds"`
	Sync    []ConfigSync `json:"sync"`
}

// ConfigCred specifies the details for a registry that container images may
// be pulled from or pushed to.
type ConfigCred struct {
	Registry string `json:"registry"`
	User     string `json:"user"`
	Pass     string `json:"pass"`
}

// ConfigSync defines a source/target repository to sync.
type ConfigSync struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

func ReadConfig(fileName string) (Config, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read: %w", err)
	}

	config := Config{}
	if err := yaml.UnmarshalStrict(contents, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return config, nil
}

func WriteConfig(fileName string, config Config) error {
	contents := []byte("##################################################\n" +
		"# THIS FILE IS AUTO-GENERATED. DO NOT MODIFY IT.\n" +
		"##################################################\n")
	yamlContents, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	contents = append(contents, yamlContents...)

	if err := os.WriteFile(fileName, contents, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
