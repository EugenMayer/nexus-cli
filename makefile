ci-build:
	rm -rf dist
	mkdir -p dist
	dep ensure -v
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags netgo -o dist/nexus-cli-linux main.go sorter.go
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -tags netgo -o dist/nexus-cli-osx main.go sorter.go
	ls dist/
	chmod +x dist/*

# / this tasks are used inside the docker build image and ci! /

# local docker based build, like in concourse
build-docker:
	#docker stop drupalwikihost > /dev/null 2>&1 || true
	docker build --rm -t nexus-cli-builder .
	docker run -v `pwd`/dist:/dist --rm --entrypoint /bin/sh nexus-cli-builder -c 'cp /go/src/github.com/eugenmayer/nexus-cli/dist/nexus-cli* /dist/'

test: init
	CGO_ENABLED=0 go test -tags netgo test/*.go

init:
	brew install dep
	dep ensure