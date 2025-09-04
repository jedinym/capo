package main

import (
	"log"
	"strings"
)

type CopyMask struct {
	mask map[string][]string
}

func NewCopyMask(builders []Builder) CopyMask {
	if len(builders) == 0 {
		log.Panicln("Cannot create CopyMask if no Builders provided!")
	}
	topBuilder := builders[len(builders)-1]

	graphs := make([]copyNode, 0)
	for _, copy := range topBuilder.copies {
		root := copyNode{
			builder:  topBuilder.alias,
			source:   copy.source,
			dest:     copy.dest,
			children: make([]copyNode, 0),
		}
		buildDependencyTree(&root, builders, len(builders)-1)
		graphs = append(graphs, root)
	}

	mask := make(map[string][]string)
	for _, tree := range graphs {
		collectCopiesInTree(tree, mask)
	}

	return CopyMask{mask: mask}
}

func (mask CopyMask) Includes(alias string, path string) bool {
	sources := mask.mask[alias]
	for _, src := range sources {
		if strings.HasPrefix("/" + path, src) {
			log.Printf("Including %s\n", path)
			return true
		}
	}

	return false
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
