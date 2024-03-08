BUILD_CHANNEL?=local
OS=$(shell uname)
VERSION=v1.12.0
GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell git tag --points-at | sort -Vr | head -n1)
GO_BUILD_LDFLAGS = -ldflags "-X 'main.Version=${TAG_VERSION}' -X 'main.GitRevision=${GIT_REVISION}'"
GOPATH = $(HOME)/go/bin
export PATH := ${GOPATH}:$(PATH) 
SHELL := /usr/bin/env bash 

default: setup build

setup: install-dependencies

install-dependencies:
ifneq (, $(shell which brew))
	brew update
else ifneq (, $(shell which apt-get))
	sudo apt-get update
else
	$(error "Unsupported system. Only apt and brew currently supported.")
endif

goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	goimports -w -local=go.viam.com/utils `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

lint: goformat
	go install github.com/edaniels/golinters/cmd/combined
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/polyfloyd/go-errorlint
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs `go env GOPATH`/bin/go-errorlint -errorf
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v --config=./etc/.golangci.yaml

build:
	mkdir -p bin &&  go build $(GO_BUILD_LDFLAGS) -o bin/viam-kuka-module module.go

install:
	sudo cp bin/viam-kuka-module /usr/local/bin/viam-kuka-module

test: 
	go test -v -coverprofile=coverage.txt -covermode=atomic ./... -race

clean: 
	rm -rf bin
	rm -f module.tar.gz

module: clean setup build appimage
	cp etc/packaging/appimages/deploy/*.AppImage viam-kuka-module.AppImage
	chmod +x viam-kuka-module.AppImage
	tar czf module.tar.gz viam-kuka-module.AppImage
	rm -f viam-kuka-module.AppImage

include *.make