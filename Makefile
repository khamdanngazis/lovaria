# Lovoria — task runner. Jalankan `make help` untuk daftar target.

SHELL := /bin/bash

# Muat .env bila ada (untuk dev lokal).
-include .env
export

TAILWIND_VERSION ?= v4.3.3
SQLC_VERSION     ?= 1.31.1
AIR_VERSION      ?= v1.67.4
GOLANGCI_VERSION ?= v2.14.0

BIN_DIR   := bin
TAILWIND  := $(BIN_DIR)/tailwindcss
SQLC      := $(BIN_DIR)/sqlc
TEMPL     := go tool templ
AIR       := go run github.com/air-verse/air@$(AIR_VERSION)
GOLANGCI  := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
LOVORIA   := go run ./cmd/server

CSS_IN    := src/styles/app.css
CSS_OUT   := static/css/app.css
MIGRATIONS_DIR := migrations
VERSION   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

# Postgres lokal dari docker-compose.yml (port 5433).
DATABASE_URL_TEST ?= postgres://lovoria:lovoria@localhost:5433/lovoria?sslmode=disable

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Darwin)
  TW_OS := macos
  SQLC_OS := darwin
else
  TW_OS := linux
  SQLC_OS := linux
endif
ifneq (,$(filter $(UNAME_M),arm64 aarch64))
  TW_ARCH := arm64
  SQLC_ARCH := arm64
else
  TW_ARCH := x64
  SQLC_ARCH := amd64
endif

.PHONY: help
help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

$(TAILWIND):
	@mkdir -p $(BIN_DIR)
	curl -sSfL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TW_OS)-$(TW_ARCH)
	chmod +x $@

$(SQLC):
	@mkdir -p $(BIN_DIR)
	curl -sSfL https://github.com/sqlc-dev/sqlc/releases/download/v$(SQLC_VERSION)/sqlc_$(SQLC_VERSION)_$(SQLC_OS)_$(SQLC_ARCH).tar.gz | tar -xz -C $(BIN_DIR) sqlc

.PHONY: tools
tools: $(TAILWIND) $(SQLC) ## Unduh Tailwind & sqlc ke ./bin

# ---------- Kode generate ----------

.PHONY: generate
generate: sqlc ## Generate kode templ + sqlc
	$(TEMPL) generate

.PHONY: sqlc
sqlc: $(SQLC) ## Generate query sqlc per modul
	@if grep -qE '^\s*- engine:' sqlc.yaml; then $(SQLC) generate; else echo "sqlc: belum ada modul terdaftar di sqlc.yaml"; fi

.PHONY: css
css: $(TAILWIND) ## Build CSS Tailwind (minified)
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --minify

# ---------- Dev ----------

.PHONY: dev
dev: $(TAILWIND) ## Jalankan server + hot reload templ & tailwind (buka http://localhost:8090)
	@$(MAKE) --no-print-directory -j2 dev-css dev-server

.PHONY: dev-css
dev-css:
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --watch=always

.PHONY: dev-server
dev-server:
	$(AIR) -c .air.toml

.PHONY: db-up
db-up: ## Jalankan Postgres lokal (docker compose, port 5433)
	docker compose up -d --wait postgres

.PHONY: db-down
db-down: ## Hentikan Postgres lokal
	docker compose down

.PHONY: seed
seed: ## Isi data contoh (development)
	$(LOVORIA) seed

# ---------- Build ----------

.PHONY: build
build: generate css ## Build binary ke ./bin/lovoria
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN_DIR)/lovoria ./cmd/server

.PHONY: run
run: build ## Build lalu jalankan binary
	./$(BIN_DIR)/lovoria

.PHONY: docker-build
docker-build: ## Build image Docker
	docker build --build-arg VERSION=$(VERSION) -t lovoria:$(VERSION) .

# ---------- Kualitas ----------

.PHONY: test
test: ## Jalankan semua test (integration test DB di-skip bila DATABASE_URL_TEST kosong)
	go test -race ./...

.PHONY: test-integration
test-integration: db-up ## Jalankan semua test termasuk integration test Postgres
	DATABASE_URL_TEST="$(DATABASE_URL_TEST)" LOVORIA_REQUIRE_DB_TESTS=true go test -race -count=1 ./...

.PHONY: lint
lint: lint-tenant ## gofmt, templ fmt, go vet, golangci-lint, lint-tenant
	@test -z "$$(gofmt -l $$(git ls-files '*.go' | grep -v '_templ.go$$'))" || (gofmt -l . && echo "jalankan: gofmt -w ." && exit 1)
	$(TEMPL) fmt -fail .
	go vet ./...
	$(GOLANGCI) run

.PHONY: lint-tenant
lint-tenant: ## Cek query tenant memfilter wedding_id & wedding_id ter-index
	@go run ./tools/linttenant

.PHONY: generate-check
generate-check: generate ## Pastikan hasil templ/sqlc generate sudah di-commit
	@git diff --exit-code -- '*_templ.go' 'src/modules/*/db/*.go' || (echo "hasil generate belum di-commit" && exit 1)
	@test -z "$$(git ls-files --others --exclude-standard -- '*_templ.go' 'src/modules/*/db/*.go')" || (echo "ada file generate baru yang belum di-commit" && exit 1)

# ---------- Migration ----------

.PHONY: migrate-up
migrate-up: ## Jalankan migration (butuh DATABASE_URL)
	$(LOVORIA) migrate up

.PHONY: migrate-down
migrate-down: ## Rollback 1 migration (butuh DATABASE_URL)
	$(LOVORIA) migrate down

.PHONY: migrate-status
migrate-status: ## Tampilkan status migration
	$(LOVORIA) migrate status

.PHONY: migrate-new
migrate-new: ## Buat file migration baru: make migrate-new name=create_weddings
	@test -n "$(name)" || (echo "pakai: make migrate-new name=create_xxx" && exit 1)
	@last=$$(ls $(MIGRATIONS_DIR)/*.sql 2>/dev/null | sed -E 's#.*/0*([0-9]+)_.*#\1#' | sort -n | tail -1); \
	f=$(MIGRATIONS_DIR)/$$(printf '%05d' $$(( $${last:-0} + 1 ))_$(name).sql); \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > $$f; echo "dibuat: $$f"

.PHONY: clean
clean: ## Hapus artefak build
	rm -rf tmp $(BIN_DIR)/lovoria $(CSS_OUT)
