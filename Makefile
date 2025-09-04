.PHONY:
run:
	buildah unshare go run .

.PHONY:
debug:
	buildah unshare dlv debug

.PHONY:
test:
	go test .
