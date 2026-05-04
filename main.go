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

var locations map[string]map[string][2]float64

const (
	author = "Piotr Pręciuk"
	port   = 6767
)

func main() {
	healthFlag := flag.Bool("healthcheck", false, "healthcheck")
	flag.Parse()

	if err := json.Unmarshal(locationsJSON, &locations); err != nil {
		log.Fatalf("locations.json: %v", err)
	}

	if *healthFlag {
		os.Exit(runHealthcheck())
	}

	log.SetFlags(0)
	log.Printf("started_at=%s author=%q port=%d", time.Now().UTC().Format(time.RFC3339), author, port)

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

func runHealthcheck() int {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 1
	}
	return 0
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func handleLocations(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(locations)
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	city := r.URL.Query().Get("city")
	if country == "" || city == "" {
		writeJSONErr(w, http.StatusBadRequest, "missing country or city")
		return
	}
	cities, ok := locations[country]
	if !ok {
		writeJSONErr(w, http.StatusBadRequest, "unknown country")
		return
	}
	coords, ok := cities[city]
	if !ok {
		writeJSONErr(w, http.StatusBadRequest, "unknown city")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := fetchWeather(ctx, coords[0], coords[1])
	if err != nil {
		writeJSONErr(w, http.StatusBadGateway, err.Error())
		return
	}
	out := map[string]any{
		"country":     country,
		"city":        city,
		"latitude":    coords[0],
		"longitude":   coords[1],
		"temperature": data.Current.Temperature,
		"apparent":    data.Current.Apparent,
		"humidity":    data.Current.Humidity,
		"wind":        data.Current.Wind,
		"description": describeWeatherCode(data.Current.WeatherCode),
		"time":        data.Current.Time,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(out)
}

func writeJSONErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type openMeteoResponse struct {
	Current struct {
		Time        string  `json:"time"`
		Temperature float64 `json:"temperature_2m"`
		Apparent    float64 `json:"apparent_temperature"`
		Humidity    int     `json:"relative_humidity_2m"`
		WeatherCode int     `json:"weather_code"`
		Wind        float64 `json:"wind_speed_10m"`
	} `json:"current"`
}

func fetchWeather(ctx context.Context, lat, lon float64) (*openMeteoResponse, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m")
	q.Set("timezone", "auto")
	u := "https://api.open-meteo.com/v1/forecast?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, string(body))
	}
	var out openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func describeWeatherCode(c int) string {
	switch c {
	case 0:
		return "bezchmurnie"
	case 1, 2:
		return "częściowe zachmurzenie"
	case 3:
		return "zachmurzenie"
	case 45, 48:
		return "mgła"
	case 51, 53, 55:
		return "mżawka"
	case 56, 57:
		return "marznąca mżawka"
	case 61, 63, 65:
		return "deszcz"
	case 66, 67:
		return "marznący deszcz"
	case 71, 73, 75:
		return "śnieg"
	case 77:
		return "ziarna śniegu"
	case 80, 81, 82:
		return "przelotny deszcz"
	case 85, 86:
		return "przelotny śnieg"
	case 95:
		return "burza"
	case 96, 99:
		return "burza z gradem"
	default:
		return fmt.Sprintf("kod WMO %d", c)
	}
}
