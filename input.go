package main

import "fmt"

const FINAL_STAGE string = ""

// TODO: create a pair of Containerfile and the resulting data structure as an example

// Input is a representation of COPY-ies from builder and external images.
// Parsed from the output of the dockerfile-json tool.
type Input struct {
	builders  []Builder
	externals []External
}

// Builder represents a named stage (AS <alias>) in the Containerfile.
type Builder struct {
	// Pullspec of the builder image.
	pullspec string
	// Alias of the builder stage.
	alias string
	// Slice of copies from this builder image.
	// NOT the copies in this builder stage
	copies []Copy
}

// External represents an external image that is copied FROM in the Containerfile.
// E.g. "COPY --from=quay.io/konflux-ci/mobster:123 src/ dest/"
type External struct {
	// Pullspec of the external image.
	pullspec string
	// Slice of copies from this external image.
	copies []Copy
}

// Copy represents a COPY command, excepting copies from context (only external image and builder copies).
type Copy struct {
	source []string
	dest   string
	// Alias of the builder stage this COPY is found in or FINAL_STAGE if copying from final
	stage string
}

func (c Copy) IsFromFinalStage() bool {
	return c.stage == FINAL_STAGE
}

// ParseInput takes the path to a dockerfile-json output file and
// parses it into the internal representation.
func ParseInput(path string) (Input, error) {
	// TODO: implement
	return Input{}, fmt.Errorf("Not implemented")
}
