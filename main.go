package main

import (
	"fmt"
	"log"

	"go.podman.io/storage"
)

func main() {
	// TODO: add external image processing
	input := Input{
		builders: []Builder{
			{
				pullspec: "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
				alias:    "first",
				copies: []Copy{
					{
						source: []string{"/dir", "/dir2"},
						dest:   "/dest/",
					},
				},
			},
			{
				pullspec: "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
				alias:    "last",
				copies: []Copy{
					{
						source: []string{"/dest/"},
						dest:   "/app",
					},
				},
			},
		},
	}

	mask := NewCopyMask(input.builders)

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
