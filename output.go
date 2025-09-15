package main

import (
	"encoding/json"
	"os"
	"path"
)

type Index struct {
	Builder  []BuilderImage  `json:"builder"`
	External []ExternalImage `json:"external"`
}

type BuilderImage struct {
	Pullspec         string `json:"pullspec"`
	IntermediateSBOM string `json:"intermediate_sbom,omitempty"`
	BuilderSBOM      string `json:"builder_sbom"`
}

type ExternalImage struct {
	Pullspec string `json:"pullspec"`
	SBOM     string `json:"sbom"`
}

func (i *Index) Write(output string) (string, error) {
	iPath := path.Join(output, "index.json")
	f, err := os.Create(iPath)
	if err != nil {
		return iPath, err
	}

	encoder := json.NewEncoder(f)
	return iPath, encoder.Encode(i)
}
