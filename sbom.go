package main

import (
	"context"
	"os"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format/spdxjson"
	"github.com/anchore/syft/syft/source/sourceproviders"
	_ "modernc.org/sqlite" // required for Syft's RPM cataloguer
)

// TODO: create a wrapper struct that sets up the common configs
func SyftScan(root string, dest string) error {
	ctx := context.Background()

	srcConfig := syft.DefaultGetSourceConfig().WithSources(sourceproviders.DirTag)

	src, err := syft.GetSource(ctx, root, srcConfig)
	if err != nil {
		return err
	}

	sbom, err := syft.CreateSBOM(ctx, src, syft.DefaultCreateSBOMConfig())
	if err != nil {
		return err
	}

	encoder, err := spdxjson.NewFormatEncoderWithConfig(spdxjson.DefaultEncoderConfig())
	if err != nil {
		return err
	}

	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	return encoder.Encode(file, *sbom)
}
