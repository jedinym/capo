package main

import (
	"log"

	"go.podman.io/storage"
	"go.podman.io/storage/pkg/reexec"
)

func main() {
	// TODO: add external image processing
	input := Input{
		builders: []Builder{
			{
				pullspec: "quay.io/konflux-ci/oras:1bd29cc",
				alias:    "builder",
				copies: []Copy{
					{
						source: []string{"/usr/bin/oras"},
						dest:   "/usr/bin/oras",
					},
				},
			},
		},
	}

	// The containers/storage library requires this to run for some operations
	if reexec.Init() {
		log.Fatalln("Failed to init reexec")
	}

	masks := NewCopyMasks(input.builders)
	log.Printf("Parsed copy masks: %+v\n", masks)

	opts, err := storage.DefaultStoreOptions()
	if err != nil {
		log.Fatalln("Failed to create default container storage options")
	}

	store, err := storage.GetStore(opts)
	if err != nil {
		log.Fatalln("Failed to create container storage")
	}

	builderData := make([]BuilderImage, 0)

	output := "./output"
	for _, builder := range input.builders {
		data, err := ProcessBuilder(store, output, builder, masks[builder.alias])
		if err != nil {
			log.Fatalf("Failed to process builder %+v with error: %v\n", builder, err)
		}
		builderData = append(builderData, data)
	}

	index := Index{
		Builder: builderData,
	}
	iPath, err := index.Write(output)
	if err != nil {
		log.Fatalln("Failed to write index to %s with error: %v\n", iPath, err)
	}

	log.Printf("Written index to %s", iPath)
}
