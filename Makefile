BIN := bin/liszt
PKG := ./cmd/liszt

.PHONY: build vet clean

build:
	go build -o $(BIN) $(PKG)

vet:
	go vet ./cmd/liszt ./internal/...

clean:
	rm -rf bin
