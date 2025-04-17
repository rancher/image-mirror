package legacy

import (
	"fmt"
	"os"
	"strings"
)

type ImagesListEntry struct {
	Source string
	Target string
	Tag    string
}

func ParseImagesList(fileName string) (string, []ImagesListEntry, error) {
	comment := ""
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read: %s", err)
	}

	lines := strings.Split(string(contents), "\n")
	imagesList := make([]ImagesListEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			comment = comment + line + "\n"
			continue
		}
		elements := strings.Split(line, " ")
		if len(elements) != 3 {
			return "", nil, fmt.Errorf("line %q does not have 3 elements", line)
		}
		imagesListEntry := ImagesListEntry{
			Source: elements[0],
			Target: elements[1],
			Tag:    elements[2],
		}
		imagesList = append(imagesList, imagesListEntry)
	}

	return comment, imagesList, nil
}

func WriteImagesList(fileName string, comment string, imagesList []ImagesListEntry) error {
	fd, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fd.Close()

	if _, err := fmt.Fprint(fd, comment); err != nil {
		return fmt.Errorf("failed to write comment: %w", err)
	}

	for _, imagesListEntry := range imagesList {
		line := fmt.Sprintf("%s %s %s", imagesListEntry.Source, imagesListEntry.Target, imagesListEntry.Tag)
		if _, err := fmt.Fprintln(fd, line); err != nil {
			return fmt.Errorf("failed to write line %q: %w", line, err)
		}
	}

	return nil
}
