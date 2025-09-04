package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"

	"go.podman.io/storage"
	"go.podman.io/storage/pkg/archive"
)

func findImage(images []storage.Image, pullspec string) (storage.Image, error) {
	for _, image := range images {
		if slices.Contains(image.Names, pullspec) {
			return image, nil
		}
	}

	return storage.Image{}, fmt.Errorf("Could not find image %s", pullspec)
}

func getBuilderLayer(store storage.Store, pullspec string) (*storage.Layer, error) {
	images, err := store.Images()
	if err != nil {
		return nil, err
	}

	i, err := findImage(images, pullspec)
	if err != nil {
		return nil, err
	}

	return store.Layer(i.TopLayer)
}

func getIntermediateLayers(store storage.Store, builderLayer *storage.Layer) ([]*storage.Layer, error) {
	images, err := store.Images()
	if err != nil {
		return nil, err
	}

	var candidates []*storage.Layer

	for _, img := range images {
		// The image for the last intermediate layer never has a name
		if len(img.Names) != 0 {
			continue
		}
		// This is an image for the builder layer itself
		if img.TopLayer == builderLayer.ID {
			continue
		}

		imgTopLayer, err := store.Layer(img.TopLayer)
		if err != nil {
			return nil, err
		}

		layerId := img.TopLayer

		for {
			if layerId == "" {
				break
			}
			if layerId == builderLayer.ID {
				candidates = append(candidates, imgTopLayer)
				break
			}

			layer, err := store.Layer(layerId)
			if err != nil {
				return nil, err
			}

			layerId = layer.Parent
		}
	}

	return candidates, nil
}

func getLastIntermediateLayer(store storage.Store, builderLayer *storage.Layer) (*storage.Layer, error) {
	candidates, err := getIntermediateLayers(store, builderLayer)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		// TODO: this might also suggest that there is no layer created in the builder
		return nil, fmt.Errorf("Could not find last intermediate layer")
	}

	// Find the candidate with the longest layer chain (furthest from builder)
	var longestChain *storage.Layer
	maxDepth := 0

	for _, candidate := range candidates {
		depth := 0
		layerId := candidate.ID

		for {
			if layerId == "" {
				break
			}
			if layerId == builderLayer.ID {
				break
			}

			layer, err := store.Layer(layerId)
			if err != nil {
				return nil, err
			}

			depth++
			layerId = layer.Parent
		}

		if depth > maxDepth {
			maxDepth = depth
			longestChain = candidate
		}
	}

	return longestChain, nil
}

func saveDiff(store storage.Store, dest string, layerId string, parentId string, alias string, mask CopyMask) error {
	compression := archive.Uncompressed
	opts := storage.DiffOptions{
		Compression: &compression,
	}

	diff, err := store.Diff(parentId, layerId, &opts)
	if err != nil {
		return err
	}
	defer diff.Close()

	reader := tar.NewReader(diff)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if !mask.Includes(alias, header.Name) {
			continue
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, reader); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
		// TODO: do we need to handle symlinks?
	}

	return nil
}

func saveDiffs(store storage.Store, alias string, interLayerId string, builderLayerId string, mask CopyMask) (string, string, error) {
	builderPath, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}
	log.Printf("builder path: %s for layer %s\n", builderPath, builderLayerId)

	interPath, err := os.MkdirTemp("", "")
	if err != nil {
		return "", "", err
	}
	log.Printf("intermediate path: %s for layer %s\n", interPath, interLayerId)

	err = saveDiff(store, builderPath, builderLayerId, "", alias, mask)
	if err != nil {
		return "", "", err
	}

	err = saveDiff(store, interPath, interLayerId, builderLayerId, alias, mask)
	if err != nil {
		return "", "", err
	}

	return builderPath, interPath, nil
}

func scanDiffs(dest string, bDiffPath string, iDiffPath string) (string, string, error) {
	bSbomPath := path.Join(dest, "builder.json")
	iSbomPath := path.Join(dest, "intermediate.json")

	c1 := make(chan error)
	c2 := make(chan error)

	go func() {
		c1 <- SyftScan(bDiffPath, bSbomPath)
	}()

	go func() {
		c2 <- SyftScan(iDiffPath, iSbomPath)
	}()

	if err := <-c1; err != nil {
		return "", "", err
	}

	if err := <-c2; err != nil {
		return "", "", err
	}

	return bSbomPath, iSbomPath, nil
}

func ProcessBuilder(store storage.Store, output string, builder Builder, mask CopyMask) (BuilderImage, error) {
	builderLayer, err := getBuilderLayer(store, builder.pullspec)
	if err != nil {
		return BuilderImage{}, err
	}

	interLayer, err := getLastIntermediateLayer(store, builderLayer)
	if err != nil {
		return BuilderImage{}, err
	}

	// TODO: call os.RemoveAll on the diff directories once debugging is complete
	bDiffPath, iDiffPath, err := saveDiffs(store, builder.alias, interLayer.ID, builderLayer.ID, mask)
	if err != nil {
		return BuilderImage{}, nil
	}
	//defer os.RemoveAll(bDiffPath)
	//defer os.RemoveAll(iDiffPath)

	dest := path.Join(output, "builder", builder.alias)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return BuilderImage{}, err
	}

	bSbomPath, iSbomPath, err := scanDiffs(dest, bDiffPath, iDiffPath)
	if err != nil {
		return BuilderImage{}, err
	}

	return BuilderImage{
		Pullspec:         builder.pullspec,
		IntermediateSBOM: iSbomPath,
		BuilderSBOM:      bSbomPath,
	}, nil
}
