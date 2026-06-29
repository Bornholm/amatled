BINARY     := bin/amatled
CMD        := ./cmd/amatled
BUNDLE     := web/dist/bundle.js
WEB_SRC    := $(shell find web/src -name '*.ts')
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run dev test lint clean package install-desktop

build: $(BUNDLE)
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) $(CMD)

run: build
	./$(BINARY) --log-level debug

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

package:
	rm -rf dist go
	goreleaser release --snapshot --clean

install-desktop: build
	install -Dm755 $(BINARY) "$(HOME)/.local/bin/amatled"
	install -Dm644 misc/packaging/linux/amatled.desktop "$(HOME)/.local/share/applications/amatled.desktop"
	install -Dm644 web/favicon.svg "$(HOME)/.local/share/icons/hicolor/scalable/apps/amatled.svg"
	install -Dm644 misc/packaging/linux/amatled.xml "$(HOME)/.local/share/mime/packages/amatled.xml"
	@echo "[*] Raccourci installé. Exécutez : update-desktop-database ~/.local/share/applications/"

clean:
	rm -f $(BINARY) $(BUNDLE)
	rm -rf dist/ go/
