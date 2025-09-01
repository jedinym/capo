package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
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
	matcher       LayerMatcher
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

	matcher, err := NewLayerMatcher(store, component)
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
		matcher:       matcher,
	}, nil
}

func (r *Resolver) Resolve(copy UnprocessedCopy) (Copy, error) {
	layer, err := r.matcher.MatchLayer(r.store, copy)
	if err != nil {
		return Copy{}, err
	}

	if copy.ctype == UnprocessedTypeExternal {
		return Copy{
			Type: CopyTypeExternal,
			Diff: "",
		}, nil
	}

	cType, err := r.classifyBuilderCopy(copy, layer)
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

func (r *Resolver) classifyBuilderCopy(copy UnprocessedCopy, layer *storage.Layer) (CopyType, error) {
	bLayer, ok := r.builderLayers[copy.from]
	if !ok {
		return "", fmt.Errorf("Could not find builder layer for %s", copy.from)
	}

	// FIXME: We should not be getting the diff more than once for the builder layers
	// There can be many calls of this
	aa, err := GetDiff(r.store, bLayer)
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

	lDiff, err := GetDiff(r.store, layer)
	if err != nil {
		return "", err
	}
	defer lDiff.Close()

	match, err := matchDiffs(bDiff, lDiff, copy.source)
	if err != nil {
		return "", err
	}

	if match {
		return CopyTypeBuilder, nil
	}

	return CopyTypeIntermediate, nil
}

func GetDiff(store storage.Store, layer *storage.Layer) (io.ReadCloser, error) {
	compression := archive.Uncompressed
	opts := storage.DiffOptions{
		Compression: &compression,
	}

	return store.Diff("", layer.ID, &opts)
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

// FIXME: this implementation is probably the wrong approach,
// untaring the diffs to temporary directories seems like a better way to compare
// Also need to take into account the actual layer
func matchDiffs(bDiff io.ReadCloser, lDiff io.ReadCloser, source []string) (bool, error) {
	lReader := tar.NewReader(lDiff)
	_, err := lReader.Next()
	if err == io.EOF {
		return false, fmt.Errorf("Found no changes in layer diff!")
	}
	if err != nil {
		return false, err
	}

	sourceMap := make(map[string]bool)
	for _, s := range source {
		sourceMap[s] = false
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

		for _, s := range source {
			raw, _ := strings.CutPrefix(s, "/")
			if raw == bHeader.Name {
				sourceMap[s] = true
			}
		}
	}

	for _, val := range sourceMap {
		if val == false {
			return false, nil
		}
	}

	return true, nil
}

func findImage(images []storage.Image, pullspec string) (storage.Image, error) {
	for _, image := range images {
		if slices.Contains(image.Names, pullspec) {
			return image, nil
		}
	}

	return storage.Image{}, fmt.Errorf("Could not find image %s", pullspec)
}
