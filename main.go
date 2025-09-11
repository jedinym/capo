package main

import (
	"fmt"
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
	if reexec.Init() {
		return
	}

	mask := NewCopyMask(input.builders)
	log.Printf("Parsed copy mask: %+v\n", mask)

	opts, err := storage.DefaultStoreOptions()
	if err != nil {
		log.Fatalln("Failed to create default container storage options")
	}

	store, err := storage.GetStore(opts)
	if err != nil {
		log.Fatalln("Failed to create container storage")
	}

	builderData := make([]BuilderImage, 0)

	for _, builder := range input.builders {
		data, err := ProcessBuilder(store, "./output", builder, mask)
		if err != nil {
			log.Fatalf("Failed to process builder %+v with error: %v\n", builder, err)
		}
		builderData = append(builderData, data)
	}

	index := Index{
		Builder: builderData,
	}

	fmt.Printf("%+v\n", index)
}
