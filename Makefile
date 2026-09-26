# Lovoria — task runner. Jalankan `make help` untuk daftar target.

SHELL := /bin/bash

# Muat .env bila ada (untuk dev lokal).
-include .env
export

TAILWIND_VERSION ?= v4.3.3
AIR_VERSION      ?= v1.67.4
GOOSE_VERSION    ?= v3.28.0
GOLANGCI_VERSION ?= v2.14.0

BIN_DIR   := bin
TAILWIND  := $(BIN_DIR)/tailwindcss
TEMPL     := go tool templ
AIR       := go run github.com/air-verse/air@$(AIR_VERSION)
GOOSE     := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
GOLANGCI  := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

CSS_IN    := src/styles/app.css
CSS_OUT   := static/css/app.css
MIGRATIONS_DIR := migrations
VERSION   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Darwin)
  TW_OS := macos
else
  TW_OS := linux
endif
ifneq (,$(filter $(UNAME_M),arm64 aarch64))
  TW_ARCH := arm64
else
  TW_ARCH := x64
endif

.PHONY: help
help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

$(TAILWIND):
	@mkdir -p $(BIN_DIR)
	curl -sSfL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TW_OS)-$(TW_ARCH)
	chmod +x $@

.PHONY: tools
tools: $(TAILWIND) ## Unduh Tailwind standalone CLI ke ./bin

.PHONY: generate
generate: ## Generate kode templ
	$(TEMPL) generate

.PHONY: css
css: $(TAILWIND) ## Build CSS Tailwind (minified)
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --minify

.PHONY: dev
dev: $(TAILWIND) ## Jalankan server + hot reload templ & tailwind (buka http://localhost:8090)
	@$(MAKE) --no-print-directory -j2 dev-css dev-server

.PHONY: dev-css
dev-css:
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --watch=always

.PHONY: dev-server
dev-server:
	$(AIR) -c .air.toml

.PHONY: build
build: generate css ## Build binary ke ./bin/server
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN_DIR)/server ./cmd/server

.PHONY: run
run: build ## Build lalu jalankan binary
	./$(BIN_DIR)/server

.PHONY: test
test: ## Jalankan semua test
	go test -race ./...

.PHONY: lint
lint: ## gofmt, templ fmt, go vet, golangci-lint
	@test -z "$$(gofmt -l $$(git ls-files '*.go' | grep -v '_templ.go$$'))" || (gofmt -l . && echo "jalankan: gofmt -w ." && exit 1)
	$(TEMPL) fmt -fail .
	go vet ./...
	$(GOLANGCI) run

.PHONY: templ-check
templ-check: generate ## Pastikan hasil templ generate sudah di-commit
	@git diff --exit-code -- '*_templ.go' || (echo "hasil templ generate belum di-commit" && exit 1)
	@test -z "$$(git ls-files --others --exclude-standard -- '*_templ.go')" || (echo "ada *_templ.go baru yang belum di-commit" && exit 1)

.PHONY: migrate-up
migrate-up: ## Jalankan migration (butuh DATABASE_URL)
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Rollback 1 migration (butuh DATABASE_URL)
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

.PHONY: migrate-new
migrate-new: ## Buat file migration baru: make migrate-new name=create_weddings
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(name) sql

.PHONY: docker-build
docker-build: ## Build image Docker
	docker build --build-arg VERSION=$(VERSION) -t lovoria:$(VERSION) .

.PHONY: clean
clean: ## Hapus artefak build
	rm -rf tmp $(BIN_DIR)/server $(CSS_OUT)
