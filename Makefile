.PHONY: build test vet fmt

build:
	go build -o dist/turn-broker ./cmd/turn-broker

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
