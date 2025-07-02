package config

import (
	"fmt"
	"slices"
)

type ImageIndex struct {
	SourceImage     string
	TargetImageName string
}

type ImageAccumulator struct {
	mapping map[ImageIndex]*Image
}

func NewImageAccumulator() *ImageAccumulator {
	return &ImageAccumulator{
		mapping: map[ImageIndex]*Image{},
	}
}

func (ia *ImageAccumulator) AddImages(newImages ...*Image) {
	for _, newImage := range newImages {
		pair := ImageIndex{
			SourceImage:     newImage.SourceImage,
			TargetImageName: newImage.TargetImageName(),
		}
		existingImage, ok := ia.mapping[pair]
		if !ok {
			ia.mapping[pair] = newImage
		} else {
			for _, newTag := range newImage.Tags {
				if !slices.Contains(existingImage.Tags, newTag) {
					existingImage.Tags = append(existingImage.Tags, newTag)
				}
			}
			ia.mapping[pair] = existingImage
		}
	}
}

// TagDifference returns an Image containing the tags of image that are
// not already accounted for in the image accumulator. If all tags of
// the image are accounted for, nil is returned for the image. The
// "difference" terminology comes from set theory.
func (ia *ImageAccumulator) TagDifference(image *Image) (*Image, error) {
	index := ImageIndex{
		SourceImage:     image.SourceImage,
		TargetImageName: image.TargetImageName(),
	}
	existingImage, ok := ia.mapping[index]
	if !ok {
		return image, nil
	}

	imageToReturn, err := NewImage(image.SourceImage, make([]string, 0, len(image.Tags)))
	if err != nil {
		return nil, fmt.Errorf("failed to construct new image from passed image: %w", err)
	}
	imageToReturn.DoNotMirror = image.DoNotMirror
	imageToReturn.SetTargetImageName(image.TargetImageName())
	for _, tag := range image.Tags {
		if !slices.Contains(existingImage.Tags, tag) {
			imageToReturn.Tags = append(imageToReturn.Tags, tag)
		}
	}
	if len(imageToReturn.Tags) == 0 {
		return nil, nil
	}
	return imageToReturn, nil
}

func (ia *ImageAccumulator) Images() []*Image {
	images := make([]*Image, 0, len(ia.mapping))
	for _, image := range ia.mapping {
		images = append(images, image)
	}
	return images
}
