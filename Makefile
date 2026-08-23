BINARY := pdg
BIN_DIR := bin
CMD := ./cmd/pdg

.PHONY: build run test vet fmt check clean web-install web-build

build:
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

check: fmt vet test

clean:
	rm -rf $(BIN_DIR)

web-install:
	cd web && pnpm install

web-build:
	cd web && pnpm build
