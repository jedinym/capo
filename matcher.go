package main

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"

	"go.podman.io/storage"
)

type LayerMatcher struct {
	cLayers []*storage.Layer
	matched []bool
}

func NewLayerMatcher(store storage.Store, component storage.Image) (LayerMatcher, error) {
	cLayers, err := initComponentLayers(store, component)
	if err != nil {
		return LayerMatcher{}, err
	}
	matched := make([]bool, len(cLayers))

	return LayerMatcher{
		cLayers: cLayers,
		matched: matched,
	}, nil
}

// MatchLayer tries to match a COPY command to the layer that implemented it
func (lm *LayerMatcher) MatchLayer(store storage.Store, copy UnprocessedCopy) (*storage.Layer, error) {
	for i, layer := range lm.cLayers {
		// skip this layer if it was already matched to a COPY
		if lm.matched[i] {
			continue
		}

		diff, err := GetDiff(store, layer)
		if err != nil {
			return nil, err
		}

		matches, err := matchDiff(diff, copy)
		if err != nil {
			diff.Close()
			return nil, err
		}
		diff.Close()

		if matches {
			lm.matched[i] = true
			return layer, nil
		}
	}

	return nil, fmt.Errorf("Could not find matching layer for copy: %+v", copy)
}

func initComponentLayers(store storage.Store, component storage.Image) ([]*storage.Layer, error) {
	cLayers := make([]*storage.Layer, 0)
	layerId := component.TopLayer
	for {
		if layerId == "" {
			break
		}

		layer, err := store.Layer(layerId)
		if err != nil {
			return nil, err
		}

		cLayers = append(cLayers, layer)
		layerId = layer.Parent
	}

	return cLayers, nil
}

func matchDiff(diff io.ReadCloser, copy UnprocessedCopy) (bool, error) {
	// TODO: this needs to be deduplicated with matchDiffs
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

		// remove '/' prefix - the layer is never in the root
		noPrefixPath, _ := strings.CutPrefix(copy.dest, "/")

		noSuffixHeaderName, _ := strings.CutSuffix(header.Name, "/")
		// TODO: this might not be enough to match, explore more options
		if noSuffixHeaderName == noPrefixPath {
			return true, nil
		}
	}

	return false, nil
}
