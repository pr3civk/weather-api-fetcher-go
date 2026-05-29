# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache upx
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY *.go index.html locations.json ./

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" -trimpath \
    -o /out/server . && \
    upx --best --lzma /out/server

FROM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch

LABEL org.opencontainers.image.authors="Piotr Pręciuk <101650@pollub.edu.pl>" \
      org.opencontainers.image.title="zad-01-weather" \
      org.opencontainers.image.description="Aplikacja pogodowa (kraj/miasto -> Open-Meteo)" \
      org.opencontainers.image.source="https://github.com/pr3civk/weather-api-fetcher-go" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="1.0.0"

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/server /server

USER 10001:10001

EXPOSE 6767

HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=3 \
    CMD ["/server", "-healthcheck"]

ENTRYPOINT ["/server"]
