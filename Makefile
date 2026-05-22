BIN := bin/liszt
PKG := ./cmd/liszt
COVER := cover.out

.PHONY: build vet test lint cover clean

build:
	go build -o $(BIN) $(PKG)

vet:
	go vet ./cmd/liszt ./internal/...

test:
	go test ./... -race -covermode=atomic -coverprofile=$(COVER)

lint:
	golangci-lint run

cover: test
	go-test-coverage --config=.testcoverage.yml

clean:
	rm -rf bin $(COVER)
