package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"

	"go.podman.io/storage"
	"go.podman.io/storage/pkg/archive"
)

type Resolver struct {
	store         storage.Store
	images        []storage.Image
	layers        []storage.Layer
	builderLayers map[string]*storage.Layer
	component     storage.Image
}

func NewResolver(pullspec string, builders []string) (Resolver, error) {
	opts, err := storage.DefaultStoreOptions()
	if err != nil {
		return Resolver{}, err
	}

	store, err := storage.GetStore(opts)
	if err != nil {
		return Resolver{}, err
	}

	images, err := store.Images()
	if err != nil {
		return Resolver{}, err
	}

	layers, err := store.Layers()
	if err != nil {
		return Resolver{}, err
	}

	component, err := findImage(images, pullspec)
	if err != nil {
		return Resolver{}, err
	}

	bLayers, err := initBuilderLayers(images, store, builders)

	return Resolver{
		store:         store,
		images:        images,
		layers:        layers,
		component:     component,
		builderLayers: bLayers,
	}, nil
}

func (r *Resolver) Resolve(copy UnprocessedCopy) (Copy, error) {

	layer, err := r.findMatchingLayer(copy)
	if err != nil {
		return Copy{}, err
	}

	cType, err := r.classifyCopy(copy, layer)
	if err != nil {
		return Copy{}, err
	}

	return Copy{
		Type: cType,
		Diff: "",
	}, nil
}

func (r *Resolver) Free() {
	r.store.Free()
}

func initBuilderLayers(images []storage.Image, store storage.Store, builders []string) (map[string]*storage.Layer, error) {
	m := make(map[string]*storage.Layer)

	for _, b := range builders {
		i, err := findImage(images, b)
		if err != nil {
			return nil, err
		}

		l, err := store.Layer(i.TopLayer)
		if err != nil {
			return nil, err
		}

		m[b] = l
	}

	return m, nil
}

func (r *Resolver) classifyCopy(copy UnprocessedCopy, layer *storage.Layer) (CopyType, error) {
	bLayer, ok := r.builderLayers[copy.from]
	if !ok {
		return "", fmt.Errorf("Could not find builder layer for %s", copy.from)
	}

	aa, err := r.getDiff(bLayer)
	if err != nil {
		return "", err
	}

	// TODO: the copying is REALLY inefficient, rethink this approach!
	// might have to untar both diffs to compare
	bDiff, err := copyStream(aa)
	if err != nil {
		aa.Close()
		return "", err
	}
	aa.Close()

	lDiff, err := r.getDiff(layer)
	if err != nil {
		return "", err
	}
	defer lDiff.Close()

	match, err := matchDiffs(bDiff, lDiff, copy.spath)
	if err != nil {
		return "", err
	}

	if match {
		return CopyTypeBuilder, nil
	}

	return CopyTypeIntermediate, nil
}

func (r *Resolver) findMatchingLayer(copy UnprocessedCopy) (*storage.Layer, error) {
	layerId := r.component.TopLayer
	for {
		if layerId == "" {
			break
		}

		layer, err := r.store.Layer(layerId)
		if err != nil {
			return nil, err
		}

		diff, err := r.getDiff(layer)
		if err != nil {
			return nil, err
		}

		matches, err := matchDiff(diff, copy.dpath)
		if err != nil {
			diff.Close()
			return nil, err
		}
		diff.Close()

		if matches {
			return layer, nil
		}

		layerId = layer.Parent
	}

	return nil, fmt.Errorf("Could not find matching layer for copy: %+v", copy)
}

func (r *Resolver) getDiff(layer *storage.Layer) (io.ReadCloser, error) {
	compression := archive.Uncompressed
	opts := storage.DiffOptions{
		Compression: &compression,
	}

	return r.store.Diff("", layer.ID, &opts)
}

// copyStream consumes the entire stream and returns a new ReadCloser with the same contents
func copyStream(src io.ReadCloser) (io.ReadCloser, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, src)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func matchDiffs(bDiff io.ReadCloser, lDiff io.ReadCloser, source string) (bool, error) {
	// FIXME: a COPY can have multiple source paths
	lReader := tar.NewReader(lDiff)
	lHeader, err := lReader.Next()
	if err == io.EOF {
		return false, fmt.Errorf("Found no changes in layer diff!")
	}
	if err != nil {
		return false, err
	}

	bReader := tar.NewReader(bDiff)
	for {
		bHeader, err := bReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		raw, _ := strings.CutPrefix(source, "/")
		if raw == bHeader.Name && matchHeaders(bHeader, lHeader) {
			return true, nil
		}
	}

	return false, nil
}

func matchHeaders(bHeader *tar.Header, lHeader *tar.Header) bool {
	if bHeader.FileInfo().IsDir() {
		log.Fatalln("Dir matching not implemented yet")
	}

	if !bHeader.ChangeTime.Equal(lHeader.ChangeTime) {
		return false
	}

	if bHeader.Size != lHeader.Size {
		return false
	}

	// TODO: this requires more testing but worst-case we might have to calculate checksums

	return true
}

func matchDiff(diff io.ReadCloser, path string) (bool, error) {
	// TODO: this also needs to support directories
	reader := tar.NewReader(diff)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		// TODO: this might not be enough to match, explore more options
		if header.Name == path {
			return true, nil
		}
	}

	return false, nil
}

func findImage(images []storage.Image, pullspec string) (storage.Image, error) {
	for _, image := range images {
		if slices.Contains(image.Names, pullspec) {
			return image, nil
		}
	}

	return storage.Image{}, fmt.Errorf("Could not find image %s", pullspec)
}
