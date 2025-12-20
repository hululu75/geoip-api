package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

var db *geoip2.Reader

type CountryResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

func main() {
	dbPath := os.Getenv("GEOIP_DB_PATH")
	if dbPath == "" {
		dbPath = "/data/GeoLite2-Country.mmdb"
	}

	var err error
	db, err = geoip2.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open GeoIP database: %v", err)
	}
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/country/", countryHandler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("GeoIP API listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "GeoIP API\n\nUsage:\n  /country/{ip}              - Returns country code (text)\n  /country/{ip}?format=json  - Returns JSON format\n\nExample:\n  /country/8.8.8.8\n  /country/8.8.8.8?format=json\n\nHealth check: /health\n")
}

func countryHandler(w http.ResponseWriter, r *http.Request) {
	// 获取 IP，去掉 /country/ 前缀
	ipStr := strings.TrimPrefix(r.URL.Path, "/country/")

	if ipStr == "" {
		http.Error(w, "Usage: /country/{ip} or /country/{ip}?format=json", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	record, err := db.Country(ip)
	if err != nil {
		country := "XX"
		respondWithFormat(w, r, ipStr, country)
		return
	}

	country := record.Country.IsoCode
	if country == "" {
		country = "XX"
	}

	respondWithFormat(w, r, ipStr, country)
}

func respondWithFormat(w http.ResponseWriter, r *http.Request, ip, country string) {
	format := r.URL.Query().Get("format")

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CountryResponse{
			IP:      ip,
			Country: country,
		})
	} else {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, country)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
