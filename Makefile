BINARY := bin/fluxexp
PKG := ./...

.PHONY: build test vet run tidy clean

build:
	go build -o $(BINARY) ./cmd/fluxexp

test:
	go test $(PKG)

vet:
	go vet $(PKG)

run:
	go run ./cmd/fluxexp $(ARGS)

tidy:
	go mod tidy

clean:
	rm -rf bin
