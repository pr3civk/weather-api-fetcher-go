# Zadanie 1 — Aplikacja pogodowa w kontenerze Docker

**Autor:** Piotr Pręciuk
**Stack:** Go (stdlib) + Open-Meteo API + Docker (multi-stage, scratch)

## Linki

- **GitHub:** https://github.com/pr3civk/weather-api-fetcher-go
- **DockerHub:** https://hub.docker.com/r/precivk69/wather-app-go

## Opis

Aplikacja webowa: użytkownik wybiera kraj (Polska / Francja / Chiny) i miasto z predefiniowanej listy (`locations.json`), klika „Sprawdź pogodę" — backend (Go) odpytuje Open-Meteo API i zwraca aktualną temperaturę, odczuwalną, wilgotność, prędkość wiatru oraz opis pogody (mapowanie kodów WMO na PL).

Cele:
- log startowy (data, autor, port) widoczny w `docker logs`,
- minimalny obraz Docker (scratch, statyczna binarka Go),
- multi-stage build z cache modułów,
- HEALTHCHECK realizowany bez `curl`/`wget` (sama binarka jako self-check via flag),
- OCI labels.

## Struktura

```
zad-01/
├── go.mod
├── main.go         — serwer HTTP + flag -healthcheck + log startowy
├── locations.json  — predefiniowana lista miast (Polska, lat/lon)
├── index.html      — UI (embedded w binarkę przez //go:embed)
├── Dockerfile      — multi-stage build, base scratch
├── .dockerignore
├── Makefile        — targety: build/run/logs/size/layers/push
└── README.md       — sprawozdanie (ten plik)
```

---

## 1. Aplikacja (max 30%)

### Funkcjonalność

- **1a. Log startowy:** `log.Printf("started_at=... author=... port=...")` — widoczny w `docker logs`.
- **1b. Wybór lokalizacji + pogoda:** dwa cascading `<select>` (kraj → miasto, lista z `locations.json`), backend proxy do Open-Meteo, wyświetla temperaturę, odczuwalną, wilgotność, wiatr, opis.

### `main.go`

```go
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

//go:embed index.html
var indexHTML []byte

//go:embed locations.json
var locationsJSON []byte

// kraj -> miasto -> [latitude, longitude]; ladowane z locations.json przy starcie
var locations map[string]map[string][2]float64

const author = "Piotr Pręciuk"

func main() {
	// flaga -healthcheck pozwala uzyc tej samej binarki jako healthcheck w scratch
	healthFlag := flag.Bool("healthcheck", false, "wewnetrzny healthcheck: GET /health -> exit 0/1")
	flag.Parse()

	if err := json.Unmarshal(locationsJSON, &locations); err != nil {
		log.Fatalf("locations.json: %v", err)
	}

	port := getPort()

	if *healthFlag {
		os.Exit(runHealthcheck(port))
	}

	// log startowy (1a): data, autor, port
	log.SetFlags(0)
	log.Printf("started_at=%s author=%q port=%d",
		time.Now().UTC().Format(time.RFC3339), author, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/locations", handleLocations)
	mux.HandleFunc("/api/weather", handleWeather)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}
```

(pełna wersja: `main.go` w repo — handlery, fetcher Open-Meteo, mapa kodów WMO→PL)

### `locations.json`

Predefiniowana lista miast (PL) — embedowana do binarki przez `//go:embed`:

```json
{
  "Polska":  { "Warszawa": [52.23, 21.01], "Kraków": [50.06, 19.94], ... },
  "Francja": { "Paryż":    [48.86,  2.35], "Lyon":   [45.76,  4.84], ... },
  "Chiny":   { "Pekin":    [39.90,116.41], "Szanghaj":[31.23,121.47], ... }
}
```

(pełna wersja w `locations.json` — 10 miast PL, 5 FR, 5 CN)

### Open-Meteo API

```
GET https://api.open-meteo.com/v1/forecast
    ?latitude=<lat>&longitude=<lon>
    &current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m
    &timezone=auto
```

Bez klucza, bez rejestracji — idealne pod kontener bez sekretów.

---

## 2. Dockerfile (max 50%)

```dockerfile
# syntax=docker/dockerfile:1.7

# ============================================================
# Etap 1 — builder: kompilacja statycznej binarki Go
# ============================================================
FROM golang:1.23-alpine AS builder
WORKDIR /src

# go.mod osobno → cache `go mod download` przezywa zmiany kodu
COPY go.mod ./
RUN go mod download

COPY *.go index.html locations.json ./

# CGO_ENABLED=0 → fully static; -s -w usuwa symbole/debug; -trimpath usuwa sciezki
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

LABEL org.opencontainers.image.authors="Piotr Pręciuk <101650@pollub.edu.pl>" \
      org.opencontainers.image.title="zad-01-weather" \
      org.opencontainers.image.description="Aplikacja pogodowa (kraj/miasto -> Open-Meteo)" \
      org.opencontainers.image.source="https://github.com/pr3civk/weather-api-fetcher-go" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="1.0.0"

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/server /server

ENV PORT=8080
EXPOSE 8080

# scratch nie ma curl/wget → healthcheck wykonuje sama binarka z flaga -healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=3 \
    CMD ["/server", "-healthcheck"]

ENTRYPOINT ["/server"]
```

### Optymalizacje

| Technika | Korzyść |
|---|---|
| **Multi-stage build (3 etapy)** | builder (golang+toolchain ~300 MB) zostaje w cache, nie trafia do final |
| **`go.mod` przed `*.go`** | zmiana kodu nie unieważnia warstwy `go mod download` (cache) |
| **`CGO_ENABLED=0`** | binarka w pełni statyczna — działa na `scratch` bez libc |
| **`-ldflags="-s -w"`** | usuwa tablicę symboli i DWARF debug → ~30% mniej |
| **`-trimpath`** | usuwa lokalne ścieżki budowy → reproducible build |
| **Base `scratch`** | brak shella, libc, package managera → ~6-8 MB total |
| **Healthcheck via flag** | `scratch` nie ma curl/wget → sama binarka robi GET /health |
| **OCI labels** | `org.opencontainers.image.authors` zgodny ze standardem |
| **`//go:embed`** | UI w binarce → 1 plik w obrazie zamiast `/server + /static/...` |
| **`.dockerignore`** | wyklucza `.git`, `*.md`, `Makefile` z build context — szybszy build |

---

## 3. Komendy (max 20%)

### a. Build obrazu

```bash
docker build -t zad-01-weather:1.0 .
```

### b. Uruchomienie kontenera

```bash
docker run -d --name weather -p 6767:6767 zad-01-weather:1.0
```

Aplikacja dostępna pod `http://localhost:6767`.

### c. Logi (data uruchomienia, autor, port)

```bash
docker logs weather
```

Przykładowy wynik:
```
started_at=2026-05-04T12:34:56Z author="Piotr Pręciuk" port=6767
```

### d. Liczba warstw i rozmiar obrazu

```bash
# rozmiar
docker images zad-01-weather:1.0
docker image inspect zad-01-weather:1.0 --format '{{.Size}}'

# liczba warstw fs
docker inspect zad-01-weather:1.0 --format '{{len .RootFS.Layers}}'

# pelna historia warstw
docker history zad-01-weather:1.0
```

Oczekiwany rozmiar: **~7-9 MB**. Liczba warstw fs: **2** (CA bundle + binarka).

### Push do DockerHub _(do wykonania ręcznie)_

```bash
docker login
docker tag zad-01-weather:1.0 <dockerhub-user>/zad-01-weather:1.0
docker tag zad-01-weather:1.0 <dockerhub-user>/zad-01-weather:latest
docker push <dockerhub-user>/zad-01-weather:1.0
docker push <dockerhub-user>/zad-01-weather:latest
```

### Skróty (Makefile)

```bash
make build      # docker build
make run        # uruchom kontener
make logs       # docker logs
make size       # rozmiar obrazu
make layers     # liczba warstw + historia
make push       # tag + push do DockerHub
make help       # pełna lista targetów
```

---

## Zrzut ekranu

_(wstaw zrzut z `http://localhost:6767` po wybraniu kraju/miasta i kliknięciu "Sprawdź pogodę")_

![screenshot](docs/screenshot.png)
