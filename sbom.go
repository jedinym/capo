package main

import (
	"context"
	"log"
	"os"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format/spdxjson"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/sourceproviders"
	_ "modernc.org/sqlite" // required for Syft's RPM cataloguer
)

// TODO: create a wrapper struct that sets up the common configs
func SyftScan(root string, dest string, excludePaths []string) error {
	ctx := context.Background()

	excludeCfg := source.ExcludeConfig{
		Paths: excludePaths,
	}

	srcConfig := syft.DefaultGetSourceConfig().WithSources(sourceproviders.DirTag).WithExcludeConfig(excludeCfg)

	src, err := syft.GetSource(ctx, root, srcConfig)
	if err != nil {
		return err
	}

	log.Printf("Generating SBOM from %s with exclude: %+v\n", root, excludeCfg)
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
