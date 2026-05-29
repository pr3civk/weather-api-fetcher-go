Zadanie 1 - Aplikacja pogodowa w kontenerze Docker

Autor: Piotr Pręciuk
Stack: Go (sama biblioteka standardowa) plus Open-Meteo API plus Docker (multi-stage, scratch)

Linki do projektu:
GitHub: https://github.com/pr3civk/weather-api-fetcher-go
DockerHub: https://hub.docker.com/r/precivk69/wather-app-go
 
Opis aplikacji

Aplikacja jest prostym serwerem webowym napisanym w Go. Użytkownik wchodzi przez przeglądarkę na stronę, wybiera kraj (do wyboru są Polska, Francja i Chiny), następnie wybiera miasto z predefiniowanej listy zapisanej w pliku locations.json i klika przycisk "Sprawdź pogodę". Backend napisany w Go odpytuje wtedy publiczne API Open-Meteo i zwraca aktualną temperaturę, temperaturę odczuwalną, wilgotność powietrza, prędkość wiatru oraz tekstowy opis pogody. Opis powstaje przez zmapowanie kodów WMO zwracanych przez API na polskie nazwy.


Struktura projektu

W katalogu zad-01 znajdują się następujące pliki. Plik go.mod to plik modułu Go. Plik main.go zawiera kod serwera HTTP, obsługę flagi -healthcheck oraz log startowy. Plik locations.json zawiera predefiniowaną listę miast z koordynatami geograficznymi. Plik index.html to interfejs użytkownika, wbudowany do binarki przez dyrektywę go:embed. Dockerfile zawiera multi-stage build kończący się obrazem scratch. Plik .dockerignore wylistowuje pliki wyłączone z kontekstu builda. Makefile zawiera skróty do najważniejszych komend takich jak build, run, logs, size, layers i push. README.md to to sprawozdanie.


Aplikacja

W kodzie main.go używam log.Printf żeby wypisać linię w stylu started_at=... author=... port=..., która pojawia się na standardowym wyjściu kontenera i jest dostępna z zewnątrz przez docker logs. Dzięki temu po starcie kontenera od razu widać datę uruchomienia, autora i numer portu na którym serwer nasłuchuje.


W main.go najważniejsze rzeczy to wbudowanie plików index.html i locations.json do binarki przez go:embed, obsługa flagi -healthcheck (jeśli flaga jest ustawiona to zamiast startować serwer, binarka wykonuje GET na /health i kończy się odpowiednim kodem wyjścia), log startowy, oraz zarejestrowanie czterech handlerów - na stronę główną, /health, /api/locations i /api/weather.

Plik locations.json zawiera słownik kraj na miasto.

Dockerfile

Dockerfile ma trzy etapy. 
- Pierwszy etap to builder oparty na obrazie golang:1.23-alpine. W nim najpierw kopiuję sam plik go.mod i wykonuję go mod download, a dopiero potem kopiuję pliki źródłowe. Następnie wywołuję go build z flagami CGO_ENABLED=0 i GOOS=linux żeby binarka była w pełni statyczna, oraz z ldflags -s -w które usuwają tablicę symboli i sekcje debug DWARF (zmniejsza to plik o około 30 procent), oraz -trimpath który usuwa lokalne ścieżki budowy z binarki. Na koniec etapu builder uruchamiam UPX z opcjami --best --lzma żeby skompresować binarkę.

Drugi etap to certs, czyli bardzo mały obraz alpine, w którym instaluję tylko pakiet ca-certificates. Potrzebuję bowiem certyfikatów CA do tego, żeby z poziomu obrazu scratch dało się wykonać HTTPS do api.open-meteo.com.

Trzeci etap to obraz oparty na scratch, czyli pustym obrazie bez żadnego systemu operacyjnego. Kopiuję do niego tylko dwa pliki. Ustawiam etykiety OCI (authors, title, description, source, licenses, version), użytkownika niesuperużytkownika 10001:10001, eksponuję port 6767 i konfiguruję HEALTHCHECK który wywołuje samą binarkę z flagą -healthcheck.

Komendy

Build obrazu wykonuje się komendą docker build --provenance=false -t zad-01-weather:1.0 . w katalogu z Dockerfile.

Uruchomienie kontenera odbywa się komendą docker run -d --name weather -p 6767:6767 zad-01-weather:1.0. Aplikacja jest wtedy dostępna pod adresem http://localhost:6767. Mapuję port 6767 z kontenera na port 6767 hosta, kontener startuje w tle z nazwą weather.

Logi z datą uruchomienia, autorem i portem dostępne są pod docker logs weather. Wynik wygląda mniej więcej tak: started_at=2026-05-04T12:34:56Z author="Piotr Pręciuk" port=6767.

Rozmiar obrazu można sprawdzić przez docker images zad-01-weather:1.0

Dla wygody zrobiłem Makefile z najważniejszymi targetami. make build wykonuje docker build, make run uruchamia kontener, make logs pokazuje docker logs, make size pokazuje rozmiar obrazu, make layers pokazuje liczbę warstw i historię, make push robi tag i push do DockerHub, a make help wyświetla pełną listę targetów.


Zrzut ekranu

Zrzut ekranu z działającej aplikacji pod adresem http://localhost:6767 po wybraniu kraju i miasta oraz kliknięciu "Sprawdź pogodę" znajduje się w pliku docs/screenshot.png.


==================================================
Zadanie 2 - Pipeline GitHub Actions (build + skan CVE + push do ghcr.io)
==================================================

Cel

Łańcuch w GitHub Actions buduje obraz kontenera na podstawie tego samego Dockerfile i kodów źródłowych co w zadaniu 1, skanuje go pod kątem podatności (CVE) i dopiero gdy obraz jest czysty, wypycha go do publicznego repozytorium obrazów autora na GitHub (ghcr.io). Definicja łańcucha znajduje się w pliku .github/workflows/build-push.yml.


Przepływ łańcucha

Łańcuch składa się z jednego joba (build-scan-push) na runnerze ubuntu-latest. Kroki wykonują się sekwencyjnie, więc niepowodzenie któregokolwiek (np. bramki CVE) przerywa cały job i blokuje push. Kolejność:

1. Checkout repozytorium.
2. setup-qemu-action - emulacja QEMU, dzięki której na runnerze amd64 można zbudować również warstwę arm64.
3. setup-buildx-action - buildx, czyli builder oparty o BuildKit (wymagany do multi-arch i cache typu registry).
4. Login do DockerHub - potrzebny do zapisu i odczytu danych cache oraz do pobrania bazy podatności przez Docker Scout.
5. Login do GHCR - przy użyciu wbudowanego GITHUB_TOKEN (nie trzeba zakładać własnego sekretu).
6. metadata-action - wyliczenie tagów i etykiet OCI obrazu docelowego.
7. Build TYLKO linux/amd64 z load=true - obraz ląduje w lokalnym daemonie, żeby dało się go zeskanować. W tym kroku zapisywane i odczytywane są dane cache (registry, mode=max) w repo na DockerHub.
8. Bramka CVE (Docker Scout, command: cves) z exit-code=true i only-severity critical,high - job pada, jeśli znaleziono podatność critical lub high, co uniemożliwia wykonanie kroku 9.
9. Build multi-arch (linux/amd64 + linux/arm64) z push=true do ghcr.io. Warstwa amd64 jest brana z cache zbudowanego w kroku 7, więc realnie dobudowywana jest głównie warstwa arm64.

Warunek z treści ("push tylko gdy brak zagrożeń") jest spełniony dzięki temu, że krok push (9) jest po bramce CVE (8) - w GitHub Actions kroki domyślnie nie uruchamiają się po niepowodzeniu poprzedniego.


a. Wsparcie dwóch architektur

W kroku push przekazuję platforms: linux/amd64,linux/arm64. buildx buduje obie architektury i publikuje w ghcr.io jeden manifest list (multi-arch), z którego klient pobiera wariant właściwy dla swojej architektury. arm64 buduje się dzięki QEMU.


b. Dane cache (eksporter + backend registry, mode=max) na DockerHub

Cache konfiguruję eksporterem typu registry:
- cache-from: type=registry,ref=<repo>:cache-<branch>
- cache-to:   type=registry,ref=<repo>:cache-<branch>,mode=max,image-manifest=true

Backendem cache jest dedykowane, publiczne repozytorium na DockerHub (docker.io/<user>/zad-01-weather-cache) - inne niż repo z samym obrazem aplikacji. mode=max powoduje eksport warstw pośrednich WSZYSTKICH etapów multi-stage builda (nie tylko warstw finalnego obrazu), więc przy kolejnych przebiegach cache'owany jest też kosztowny etap builder (go mod download, go build, UPX). image-manifest=true zapisuje cache jako zgodny z OCI manifest, co poprawia kompatybilność z DockerHub.


c. Test CVE - Docker Scout

Jako skaner wybrałem Docker Scout (docker/scout-action, command: cves). Skanowany jest obraz lokalny (local://) zbudowany w kroku 7. Parametr exit-code: true sprawia, że niezerowy wynik (znalezione podatności w wybranym zakresie) kończy krok błędem i przerywa job przed pushem. only-severity: critical,high ogranicza bramkę do poważnych podatności. Obraz finalny bazuje na scratch (brak systemu plików dystrybucji, jedynie statyczna binarka Go i certyfikaty CA), więc powierzchnia ataku i liczba potencjalnych CVE są minimalne - bramka realistycznie przechodzi.


Schemat tagowania i jego uzasadnienie

Obraz (ghcr.io) - tagi generuje docker/metadata-action:
- latest        - tylko dla gałęzi domyślnej (is_default_branch); zawsze najnowszy stabilny obraz z main.
- <branch>      - nazwa gałęzi (type=ref,event=branch); pozwala testować obrazy z gałęzi bez ruszania latest.
- <wersja>      - z gita po tagu vX.Y.Z (type=semver) w wariantach pełnym (X.Y.Z) oraz X.Y; daje stabilne, niezmienne odwołanie do konkretnego wydania.
- sha-<short>   - skrócony commit SHA (type=sha); każdy build ma unikatowy, jednoznacznie identyfikowalny tag wskazujący na konkretny commit.

Uzasadnienie: rozdzielenie tagów ruchomych (latest, <branch>) od niezmiennych (semver, sha) to standard rekomendowany m.in. w dokumentacji docker/metadata-action. Tag latest jest wygodny, ale "ruchomy", więc do produkcji/odtwarzalności używa się tagów semver lub sha, które zawsze wskazują ten sam obraz. Tag sha gwarantuje powiązanie obrazu z dokładnym commitem (audytowalność).

Cache (DockerHub) - osobny schemat, bo cache to artefakt techniczny, nie wydanie:
- cache-<branch> - jeden tag cache na gałąź (cache-main itd.).

Uzasadnienie: tagowanie cache per gałąź izoluje dane cache różnych gałęzi (build na feature-branch nie nadpisuje cache main i odwrotnie), a jednocześnie utrzymuje stały, przewidywalny ref, do którego każdy kolejny build tej samej gałęzi się podłącza (cache-from) i go aktualizuje (cache-to). Trzymanie cache w osobnym repo niż obraz aplikacji nie zaśmieca repozytorium obrazów wpisami technicznymi.


Konfiguracja sekretów / zmiennych w repozytorium

Przed uruchomieniem łańcucha w Settings repozytorium na GitHub należy ustawić:
- vars.DOCKERHUB_USERNAME (Variables) - nazwa użytkownika DockerHub (namespace repo cache).
- secrets.DOCKERHUB_TOKEN (Secrets) - Access Token z DockerHub (uprawnienia Read & Write) do logowania i zapisu cache.
GITHUB_TOKEN jest wstrzykiwany automatycznie - służy do logowania i pushu do ghcr.io (job ma uprawnienie packages: write).

Dodatkowo: trzeba raz utworzyć na DockerHub publiczne repo zad-01-weather-cache, a po pierwszym pushu obrazu - w ustawieniach pakietu na GitHub ustawić jego widoczność na Public.


Uruchomienie

Łańcuch startuje przy push na gałąź main, przy tagu vX.Y.Z oraz ręcznie (workflow_dispatch). Potwierdzenie poprawnego działania (zielony przebieg) znajduje się w zakładce Actions repozytorium.
