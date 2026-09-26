# syntax=docker/dockerfile:1

# ---- Build ----
FROM golang:1.25-bookworm AS build
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
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -H -u 10001 lovoria
COPY --from=build /out/lovoria /usr/local/bin/lovoria
ENV APP_ENV=production \
    PORT=8080
EXPOSE 8080
USER lovoria
CMD ["lovoria", "serve"]
