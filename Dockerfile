# syntax=docker/dockerfile:1

# ---- Build ----
FROM golang:1.26-bookworm AS build
WORKDIR /app

ARG TAILWIND_VERSION=v4.3.3
ARG TARGETARCH
RUN case "${TARGETARCH:-amd64}" in \
      arm64) tw=linux-arm64 ;; \
      *)     tw=linux-x64 ;; \
    esac && \
    curl -sSfL -o /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-${tw}" && \
    chmod +x /usr/local/bin/tailwindcss

ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN go tool templ generate && \
    tailwindcss -i src/styles/app.css -o static/css/app.css --minify && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/lovoria ./cmd/server

# ---- Runtime ----
# Alpine (bukan distroless) supaya pre-deploy command Railway punya shell.
FROM alpine:3.24
# postgresql18-client: pg_dump/pg_restore untuk backup (lovoria backup). Versi
# mayor HARUS sama dengan server Postgres produksi (Railway: 18) — pg_restore
# yang lebih baru menulis setting yang belum dikenal server lama.
RUN apk add --no-cache ca-certificates tzdata postgresql18-client && \
    adduser -D -H -u 10001 lovoria
COPY --from=build /out/lovoria /usr/local/bin/lovoria
ENV APP_ENV=production \
    GOMEMLIMIT=256MiB \
    PORT=8080
EXPOSE 8080
USER lovoria
CMD ["lovoria", "serve"]
