package main

import (
	"log"
	"strings"
)

// TODO: implement pattern matching for all CopyMask users
// https://docs.docker.com/reference/dockerfile/#pattern-matching
type CopyMask struct {
	sources []string
}

func NewCopyMasks(builders []Builder) map[string]CopyMask {
	if len(builders) == 0 {
		return make(map[string]CopyMask)
	}

	graphs := make([]copyNode, 0)
	for i, builder := range builders {
		for _, cp := range builder.copies {
			if cp.IsFromFinalStage() {
				root := copyNode{
					builder:  builder.alias,
					source:   cp.source,
					dest:     cp.dest,
					children: make([]copyNode, 0),
				}
				buildDependencyTree(&root, builders, i)
				graphs = append(graphs, root)
			}
		}
	}

	mask := make(map[string][]string)
	for _, tree := range graphs {
		collectCopiesInTree(tree, mask)
	}

	result := make(map[string]CopyMask)
	for alias, sources := range mask {
		result[alias] = CopyMask{sources: sources}
	}

	// ensure all builders have a mask, even if not in dependency tree
	for _, builder := range builders {
		if _, exists := result[builder.alias]; !exists {
			result[builder.alias] = CopyMask{sources: []string{}}
		}
	}

	return result
}

// path cannot be prefixed with '/', the root char is added when comparing to sources
func (mask CopyMask) Includes(path string) bool {
	for _, src := range mask.sources {
		// TODO: the log statements should be moved to the callers for more context?
		if strings.HasPrefix("/"+path, src) {
			log.Printf("Including %s\n", path)
			return true
		}
	}

	return false
}

func (mask CopyMask) GetSources() []string {
	return mask.sources
}

type copyNode struct {
	builder  string
	source   []string
	dest     string
	children []copyNode
}

func buildDependencyTree(node *copyNode, builders []Builder, currentBuilderIndex int) {
	for _, srcPath := range node.source {
		for i := range currentBuilderIndex {
			builder := builders[i]
			for _, copy := range builder.copies {
				if strings.HasPrefix(copy.dest, srcPath) {
					child := copyNode{
						builder:  builder.alias,
						source:   copy.source,
						dest:     copy.dest,
						children: make([]copyNode, 0),
					}
					buildDependencyTree(&child, builders, i)
					node.children = append(node.children, child)
				}
			}
		}
	}
}

func collectCopiesInTree(node copyNode, mask map[string][]string) {
	mask[node.builder] = append(mask[node.builder], node.source...)
	for _, child := range node.children {
		collectCopiesInTree(child, mask)
	}
}
