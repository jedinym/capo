package main

import (
	"log"
)

type UnprocessedCopyType string

const (
	UnprocessedTypeBuilder  = "builder"
	UnprocessedTypeExternal = "external"
)

// FIXME: a COPY can have multiple sources
// TODO: do we need to support --link?
type UnprocessedCopy struct {
	from  string
	spath string
	dpath string
	ctype UnprocessedCopyType
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
			from:  "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
			spath: "/usr/bin/ab",
			dpath: "ab",
			ctype: UnprocessedTypeBuilder,
		},
		{
			from:  "registry.access.redhat.com/ubi9/python-312@sha256:83b01cf47b22e6ce98a0a4802772fb3d4b7e32280e3a1b7ffcd785e01956e1cb",
			spath: "/app/content",
			dpath: "content",
			ctype: UnprocessedTypeBuilder,
		},
		{
			from:  "quay.io/konflux-ci/oras:3d83c68",
			spath: "/usr/bin/oras",
			dpath: "/usr/bin/oras",
			ctype: UnprocessedTypeExternal,
		},
	}

	for _, cmd := range cmds {
		copy, err := resolver.Resolve(cmd)
		if err != nil {
			log.Fatalln(err)
		}

		log.Printf("original: %+v\t resolved: %+v\n", cmd, copy)
	}
}
