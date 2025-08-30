package main

type CopyType string

const (
	CopyTypeBuilder      = "builder"
	CopyTypeExternal     = "external"
	CopyTypeIntermediate = "intermediate"
)

// In mobster, generate an SBOM on the diff and add
// the packages and relationships (based on type) to generated SBOM
// TODO: make sure this is JSON serializable
type Copy struct {
	Type CopyType
	Diff string
}
