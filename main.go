package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/oschwald/geoip2-golang"
)

var (
	db    *geoip2.Reader
	cache *lru.Cache[string, string]

	// Cache statistics
	cacheHits     int64
	cacheMisses   int64
	cacheEnabled  bool
	cacheCapacity int
)

type CountryResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

type StatsResponse struct {
	CacheEnabled bool    `json:"cache_enabled"`
	CacheHits    int64   `json:"cache_hits"`
	CacheMisses  int64   `json:"cache_misses"`
	CacheSize    int     `json:"cache_size,omitempty"`
	CacheHitRate float64 `json:"cache_hit_rate,omitempty"`
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

	// Initialize cache if enabled
	initCache()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/country/", countryHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/stats", statsHandler)

	log.Printf("GeoIP API listening on port %s", port)
	if cacheEnabled {
		log.Printf("Cache enabled with capacity: %d", cacheCapacity)
	} else {
		log.Printf("Cache disabled")
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initCache() {
	enabledStr := os.Getenv("CACHE_ENABLED")
	if enabledStr == "" {
		enabledStr = "false"
	}
	cacheEnabled = enabledStr == "true" || enabledStr == "1"

	if !cacheEnabled {
		return
	}

	maxSize := 10000
	if sizeStr := os.Getenv("CACHE_MAX_SIZE"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			maxSize = size
		}
	}

	var err error
	cache, err = lru.New[string, string](maxSize)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	cacheCapacity = maxSize
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "GeoIP API\n\nUsage:\n  /country/{ip}              - Returns country code (text)\n  /country/{ip}?format=json  - Returns JSON format\n\nExample:\n  /country/8.8.8.8\n  /country/8.8.8.8?format=json\n\nEndpoints:\n  /health - Health check\n  /stats  - Cache statistics\n")
}

func countryHandler(w http.ResponseWriter, r *http.Request) {
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

	country := lookupCountry(ipStr)
	respondWithFormat(w, r, ipStr, country)
}

func lookupCountry(ipStr string) string {
	// Check cache first if enabled
	if cacheEnabled {
		if country, ok := cache.Get(ipStr); ok {
			atomic.AddInt64(&cacheHits, 1)
			return country
		}
		atomic.AddInt64(&cacheMisses, 1)
	}

	// Query database
	ip := net.ParseIP(ipStr)
	record, err := db.Country(ip)
	country := "XX"

	if err == nil && record.Country.IsoCode != "" {
		country = record.Country.IsoCode
	}

	// Store in cache if enabled
	if cacheEnabled {
		cache.Add(ipStr, country)
	}

	return country
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

func statsHandler(w http.ResponseWriter, r *http.Request) {
	hits := atomic.LoadInt64(&cacheHits)
	misses := atomic.LoadInt64(&cacheMisses)
	total := hits + misses

	stats := StatsResponse{
		CacheEnabled: cacheEnabled,
		CacheHits:    hits,
		CacheMisses:  misses,
	}

	if cacheEnabled && cache != nil {
		stats.CacheSize = cache.Len()
		if total > 0 {
			stats.CacheHitRate = float64(hits) / float64(total)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
