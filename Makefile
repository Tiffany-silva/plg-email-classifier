BINARY := server

.PHONY: all setup vet test build run clean

all: build

setup:
	git config core.hooksPath .githooks 2>/dev/null || true

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -f $(BINARY)
