package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

var db *geoip2.Reader

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

	http.HandleFunc("/", handler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("GeoIP API listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	// 获取 IP，去掉开头的 /
	ipStr := strings.TrimPrefix(r.URL.Path, "/")
	
	if ipStr == "" || ipStr == "favicon.ico" {
		http.Error(w, "Usage: /{ip}", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	record, err := db.Country(ip)
	if err != nil {
		http.Error(w, "XX", http.StatusOK)
		return
	}

	country := record.Country.IsoCode
	if country == "" {
		country = "XX"
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, country)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
