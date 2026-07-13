.PHONY: build test mocks

build:
	go build -o hiro-aristech-api ./cmd

test:
	go test ./... -v

mocks:
	go tool mockery
