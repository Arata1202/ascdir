.PHONY: build test fmt fmt-check vet check

build:
	go build -o ascdir ./cmd/ascdir

test:
	go test -race -shuffle=on ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

fmt-check:
	test -z "$$(gofmt -l $$(find . -name '*.go' -type f))"

vet:
	go vet ./...

check: fmt-check test vet
