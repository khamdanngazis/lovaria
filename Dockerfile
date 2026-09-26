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
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/server ./cmd/server

# ---- Runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
ENV APP_ENV=production \
    PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
