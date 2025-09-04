package main

import "fmt"

type Input struct {
	builders  []Builder
	externals []External
}

// Builder represents a named stage (AS <alias>) in a Dockerfile.
//
// Key points:
// - Each Builder represents a named stage (AS <alias>) in the Dockerfile
// - A Builder has copies only if another stage copies FROM it
// - The copies list contains what gets copied FROM this builder by other stages
type Builder struct {
	pullspec string
	alias    string
	copies   []Copy
}

type External struct {
	pullspec string
	copies   []Copy
}

type Copy struct {
	source []string
	dest   string
}

// ParseInput takes the path to a dockerfile-json output file and
// parses it into the internal representation.
// TODO: implement
func ParseInput(path string) (Input, error) {
	return Input{}, fmt.Errorf("Not implemented")
}
