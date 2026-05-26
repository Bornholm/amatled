BINARY     := bin/amatled
CMD        := ./cmd/amatled
BUNDLE     := web/dist/bundle.js
WEB_SRC    := $(shell find web/src -name '*.ts')
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run dev test lint clean

build: $(BUNDLE)
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

dev:
	npm run dev &
	go run -ldflags "-X main.version=$(VERSION)" $(CMD)

$(BUNDLE): $(WEB_SRC) package.json tsconfig.json node_modules/.package-lock.json
	npm run build

node_modules/.package-lock.json: package.json
	npm install
	@touch node_modules/.package-lock.json

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) $(BUNDLE)
