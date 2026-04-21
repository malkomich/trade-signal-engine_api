SHELL := /bin/bash

.PHONY: test run tidy

test:
	go test ./...

run:
	go run ./cmd/api

tidy:
	go mod tidy

