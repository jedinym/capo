package main

type Index struct {
	Builder  []BuilderImage  `json:"builder"`
	External []ExternalImage `json:"external"`
}

type BuilderImage struct {
	Pullspec         string `json:"pullspec"`
	IntermediateSBOM string `json:"intermediate_sbom"`
	BuilderSBOM      string `json:"builder_sbom"`
}

type ExternalImage struct {
	Pullspec string `json:"pullspec"`
	SBOM     string `json:"sbom"`
}
