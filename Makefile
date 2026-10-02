.PHONY: build

build:
	mkdir -p build
	GOBIN=$(PWD)/build go install ./client/native ./keypair-generator ./server
