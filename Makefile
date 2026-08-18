GO ?= go
NPM ?= npm
WEB_DIR := web
WEB_EMBED_DIR := internal/webembed/dist
BIN_DIR := bin

.PHONY: web-install dev-api dev-web web-build build run test

web-install:
	cd $(WEB_DIR) && $(NPM) ci

dev-api:
	$(GO) run ./cmd/atoms-demo

dev-web:
	cd $(WEB_DIR) && $(NPM) run dev

web-build: web-install
	cd $(WEB_DIR) && $(NPM) run build
	touch $(WEB_EMBED_DIR)/.gitkeep

build: web-build
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/atoms-demo ./cmd/atoms-demo

run: build
	./$(BIN_DIR)/atoms-demo

test:
	$(GO) test ./...
