# Targets:
#
#   all:          Builds the code locally after testing
#
#   fmt:          Formats the source files
#   build:        Builds the code locally
#   vet:          Vets the code
#   lint:         Runs lint over the code (you do not need to fix everything)
#   test:         Runs the tests
#   clean:        Deletes the built file (if it exists)
#
#   install:      Builds, tests and installs the code locally

.PHONY: all fmt build vet lint test clean install race

# Sub-directories containing code to be vetted or linted
CODE = accounting conc config files imp

# The first target is always the default action if `make` is called without args
# We clean, build and install into $GOPATH so that it can just be run
all: clean fmt vet test install

fmt:
	@gofmt -w ./$*

build: export GOOS=linux
build: export GOARCH=amd64
build: clean
	@go build

vet:
	@go tool vet $(CODE)
	@go tool vet main.go

lint:
	@golint $(CODE)
	@golint main.go

test:
	@go test -v -cover ./...

clean:
	@find $(GOPATH)/bin -name import-schemas -delete
	@find . -name import-schemas -delete

install: clean
	@go install

race: clean fmt vet test
	@go build -race
	@mv import-schemas $(GOPATH)/bin/