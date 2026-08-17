.PHONY: build test fmt vet check

build:
	go build -o ascdir ./cmd/ascdir

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

vet:
	go vet ./...

check: test vet
