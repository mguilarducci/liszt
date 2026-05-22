BIN := bin/liszt
PKG := ./cmd/liszt
COVER := cover.out

.PHONY: build vet test lint cover clean

build:
	go build -o $(BIN) $(PKG)

vet:
	go vet ./cmd/liszt ./internal/...

test:
	go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=$(COVER)

lint:
	golangci-lint run

cover: test
	go tool cover -func=$(COVER)

clean:
	rm -rf bin $(COVER)
