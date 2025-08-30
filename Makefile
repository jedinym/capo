.PHONY:
run: main.go
	buildah unshare go run .

.PHONY:
debug: main.go
	buildah unshare dlv debug
