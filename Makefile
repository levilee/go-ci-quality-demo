.PHONY: fmt-check vet test build run

fmt-check:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

test:
	go test -race -coverprofile=coverage.out ./...

build:
	go build ./cmd/server

run:
	go run ./cmd/server
