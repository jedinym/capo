package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.podman.io/storage"
	"go.podman.io/storage/pkg/archive"
)


func getBuilderImage(store storage.Store, pullspec string) (*storage.Image, error) {
	imgId, err := store.Lookup(pullspec)
	if err != nil {
		return nil, fmt.Errorf("Could not find builder image: %s", pullspec)
	}

	return store.Image(imgId)
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

// TODO: is this really the best way to find the layer?
func getLastIntermediateLayer(store storage.Store, builderLayer *storage.Layer) (*storage.Layer, error) {
	candidates, err := getIntermediateLayers(store, builderLayer)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
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

// It is the caller's responsibility to clean up the returned path.
func getIntermediateDiffPath(store storage.Store, builderImage *storage.Image, builder Builder, mask CopyMask) (string, error) {
	builderLayer, err := store.Layer(builderImage.TopLayer)
	if err != nil {
		return "", err
	}

	// FIXME: interLayer can be nil (when there's no intermediate layer)
	interLayer, err := getLastIntermediateLayer(store, builderLayer)
	if err != nil {
		return "", err
	}

	interPath, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	log.Printf("intermediate path: %s for layer %s\n", interPath, interLayer.ID)

	err = saveDiff(store, interPath, interLayer.ID, builderLayer.ID, builder.alias, mask)
	if err != nil {
		return "", err
	}

	return interPath, nil
}

func getExcludedPaths(root string, mask CopyMask, alias string) ([]string, error) {
	excluded := make([]string, 0)

	// Use a queue for breadth-first traversal
	queue := []string{root}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return []string{}, err
		}

		for _, entry := range entries {
			fullPath := filepath.Join(currentPath, entry.Name())
			noRoot, _ := strings.CutPrefix(fullPath, root+"/")

			// skip if parent is already excluded
			parentExcluded := false
			for _, exc := range excluded {
				if strings.HasPrefix(fullPath, filepath.Join(root, exc)) {
					parentExcluded = true
					break
				}
			}

			if parentExcluded {
				continue
			}

			if !mask.Supersets(alias, noRoot) {
				// syft excludes must begin with "./"
				toExclude := "./" + noRoot
				if entry.IsDir() {
					toExclude = toExclude + "/**"
				}
				excluded = append(excluded, toExclude)
			} else if entry.IsDir() {
				// Only add directories to queue if they're not excluded
				queue = append(queue, fullPath)
			}
		}
	}

	return excluded, nil
}

// TODO: break up into more functions
// TODO: create a struct with often used args
func ProcessBuilder(store storage.Store, output string, builder Builder, mask CopyMask) (BuilderImage, error) {
	dest := path.Join(output, "builder", builder.alias)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return BuilderImage{}, err
	}

	builderImage, err := getBuilderImage(store, builder.pullspec)
	if err != nil {
		return BuilderImage{}, err
	}

	iDiffPath, err := getIntermediateDiffPath(store, builderImage, builder, mask)
	if err != nil {
		return BuilderImage{}, err
	}
	log.Printf("Builder %s intermediate diff path: %s", builder.alias, iDiffPath)
	//defer os.RemoveAll(iDiffPath)

	iSbomPath := path.Join(dest, "intermediate.json")
	if err := SyftScan(iDiffPath, iSbomPath, []string{}); err != nil {
		return BuilderImage{}, err
	}

	mountPath, err := store.MountImage(builderImage.ID, []string{}, "")
	if err != nil {
		return BuilderImage{}, err
	}
	defer store.UnmountImage(builderImage.ID, false)

	excluded, err := getExcludedPaths(mountPath, mask, builder.alias)
	if err != nil {
		return BuilderImage{}, err
	}

	bSbomPath := path.Join(dest, "builder.json")
	log.Printf("Builder %s content path: %s", builder.alias, mountPath + "/usr/bin")
	if err := SyftScan(mountPath, bSbomPath, excluded); err != nil {
		return BuilderImage{}, err
	}
	//defer os.RemoveAll(bContentPath)

	return BuilderImage{
		Pullspec:         builder.pullspec,
		IntermediateSBOM: iSbomPath,
		BuilderSBOM:      bSbomPath,
	}, nil
}
