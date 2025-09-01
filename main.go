package main

import (
	"log"
	"slices"
)

type UnprocessedCopyType string

const (
	UnprocessedTypeBuilder  = "builder"
	UnprocessedTypeExternal = "external"
)

// TODO: do we need to support --link?
type UnprocessedCopy struct {
	from   string
	source []string
	dest   string
	ctype  UnprocessedCopyType
}

func main() {
	builders := []string{
		"registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
	}
	componentTag := "localhost/test:latest"
	resolver, err := NewResolver(componentTag, builders)
	if err != nil {
		log.Fatalln(err)
	}
	defer resolver.Free()

	cmds := []UnprocessedCopy{
		{
			from:   "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
			source: []string{"/usr/bin/ab", "/usr/bin/apxs"},
			dest:   "/app",
			ctype:  UnprocessedTypeBuilder,
		},
		{
			from:   "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
			source: []string{"/app/content"},
			dest:   "/app/content",
			ctype:  UnprocessedTypeBuilder,
		},
	}

	// reverse the copy commands, so that they are in the same order as layers
	// (top layer first)
	// this is necessary to properly match COPY-ies to layers
	slices.Reverse(cmds)

	for _, cmd := range cmds {
		copy, err := resolver.Resolve(cmd)
		if err != nil {
			log.Fatalln(err)
		}

		log.Printf("original: %+v\t resolved: %+v\n", cmd, copy)
	}
}
