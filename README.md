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
