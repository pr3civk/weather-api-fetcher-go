# syntax=docker/dockerfile:1.7

# ============================================================
# Etap 1 — builder: kompilacja statycznej binarki Go
# ============================================================
FROM golang:1.23-alpine AS builder
WORKDIR /src

# go.mod osobno → dzieki cache `go mod download` nie powtarza sie przy zmianie kodu
COPY go.mod ./
RUN go mod download

# kod + szablon UI (embedowany do binarki przez //go:embed)
COPY *.go index.html locations.json ./

# CGO_ENABLED=0 → fully static; -s -w usuwa symbole/debug; -trimpath usuwa sciezki budowy
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" -trimpath \
    -o /out/server .

# ============================================================
# Etap 2 — certs: tylko CA bundle (potrzebny do HTTPS Open-Meteo)
# ============================================================
FROM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates

# ============================================================
# Etap 3 — final: scratch (najmniejszy mozliwy obraz)
# ============================================================
FROM scratch

# OCI labels (zgodne ze standardem opencontainers.org/image-spec)
LABEL org.opencontainers.image.authors="Piotr Pręciuk <101650@pollub.edu.pl>" \
      org.opencontainers.image.title="zad-01-weather" \
      org.opencontainers.image.description="Aplikacja pogodowa (kraj/miasto -> Open-Meteo)" \
      org.opencontainers.image.source="https://github.com/pr3civk/weather-api-fetcher-go" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="1.0.0"

# CA bundle (HTTPS) i binarka — kazda warstwa to jeden COPY = minimalna liczba warstw fs
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/server /server

EXPOSE 6767

# scratch nie ma curl/wget → healthcheck wykonuje sama binarka z flaga -healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=3 \
    CMD ["/server", "-healthcheck"]

ENTRYPOINT ["/server"]
