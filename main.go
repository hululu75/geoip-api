package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/oschwald/geoip2-golang"
)

const (
	// Maximum size for downloaded database file (100MB)
	maxDownloadSize = 100 * 1024 * 1024
	// HTTP client timeout for downloads
	httpTimeout = 5 * time.Minute
	// HTTP server timeouts to prevent slowloris attacks
	serverReadTimeout  = 10 * time.Second
	serverWriteTimeout = 10 * time.Second
	serverIdleTimeout  = 60 * time.Second
	// Graceful shutdown timeout
	shutdownTimeout = 30 * time.Second
)

var (
	dbValue    atomic.Value      // stores *geoip2.Reader
	isCityDB   atomic.Bool       // tracks if database is City (true) or Country (false)
	dbMutex    = &sync.RWMutex{} // Mutex to protect DB access during reloads
	testIP     = net.ParseIP("8.8.8.8") // Pre-parsed IP for health checks
)

// Log levels
const (
	LogLevelError = iota
	LogLevelInfo
	LogLevelDebug
)

var currentLogLevel = LogLevelInfo

type CountryResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

type CityResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	City    string `json:"city,omitempty"`
	Region  string `json:"region,omitempty"`
}

type RegionResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Region  string `json:"region,omitempty"`
}

func logError(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelError {
		log.Printf("[ERROR] "+format, v...)
	}
}

func logInfo(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

func logDebug(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func isCityDatabase(db *geoip2.Reader) (bool, error) {
	dbType := strings.ToLower(db.Metadata().DatabaseType)
	if strings.Contains(dbType, "city") {
		return true, nil
	}
	if strings.Contains(dbType, "country") {
		return false, nil
	}
	return false, fmt.Errorf("unable to detect database type: %s", db.Metadata().DatabaseType)
}

func initDatabaseType(db *geoip2.Reader) error {
	isCity, err := isCityDatabase(db)
	if err != nil {
		return err
	}

	isCityDB.Store(isCity)
	if isCity {
		logInfo("Detected GeoIP database type: City (supports country, city, region)")
	} else {
		logInfo("Detected GeoIP database type: Country (supports country only)")
	}
	return nil
}

func main() {
	// Configure log level
	logLevelStr := os.Getenv("LOG_LEVEL")
	switch strings.ToUpper(logLevelStr) {
	case "ERROR":
		currentLogLevel = LogLevelError
	case "DEBUG":
		currentLogLevel = LogLevelDebug
	case "INFO", "":
		currentLogLevel = LogLevelInfo
	default:
		currentLogLevel = LogLevelInfo
		log.Printf("[WARN] Unknown LOG_LEVEL '%s', defaulting to INFO", logLevelStr)
	}

	logDebug("Log level set to: %s", logLevelStr)

	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	dbPath := os.Getenv("GEOIP_DB_PATH") // Highest precedence
	if dbPath == "" {
		dbDir := os.Getenv("GEOIP_DB_DIR")
		if dbDir != "" {
			dbFileName := os.Getenv("GEOIP_DB_FILENAME")
			if dbFileName == "" {
				dbFileName = "GeoLite2-Country.mmdb" // Default filename if only directory is specified
			}
			dbPath = filepath.Join(dbDir, dbFileName)
		} else {
			dbPath = "/data/GeoLite2-Country.mmdb" // Global default if neither path nor dir is specified
		}
	}
	forceUpdate := os.Getenv("FORCE_DB_UPDATE") == "true"
	updateIntervalHoursStr := os.Getenv("DB_UPDATE_INTERVAL_HOURS")
	updateIntervalHours := 720 // Default to 30 days (30 * 24 hours)
	if updateIntervalHoursStr != "" {
		if i, err := strconv.Atoi(updateIntervalHoursStr); err == nil && i >= 0 {
			updateIntervalHours = i
		} else {
			logInfo("Invalid DB_UPDATE_INTERVAL_HOURS '%s', using default %d", updateIntervalHoursStr, updateIntervalHours)
		}
	}

	logDebug("Configuration - DB Path: %s, Update Interval: %d hours, Force Update: %v", dbPath, updateIntervalHours, forceUpdate)

	// Check if database needs to be downloaded or updated
	needsDownload := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logInfo("GeoIP database not found at %s.", dbPath)
		needsDownload = true
	} else if forceUpdate {
		logInfo("FORCE_DB_UPDATE is true, forcing database update.")
		needsDownload = true
	} else {
		fileInfo, err := os.Stat(dbPath)
		if err != nil {
			logError("Failed to get file info for %s: %v", dbPath, err)
			needsDownload = true
		} else {
			lastModified := fileInfo.ModTime()
			logDebug("Database file last modified: %s (age: %.1f hours)", lastModified.Format(time.RFC3339), time.Since(lastModified).Hours())
			if time.Since(lastModified) > time.Duration(updateIntervalHours)*time.Hour {
				logInfo("GeoIP database at %s is older than %d hours, initiating update.", dbPath, updateIntervalHours)
				needsDownload = true
			}
		}
	}

	if needsDownload {
		if licenseKey == "" {
			log.Fatalf("MAXMIND_LICENSE_KEY not set. Cannot download or update GeoIP database. Please set the environment variable.")
		}
		logInfo("Starting GeoIP database download and verification.")
		updated, err := downloadGeoLite2DB(licenseKey, dbPath)
		if err != nil {
			log.Fatalf("Failed to download or verify GeoIP database: %v", err)
		}
		if updated {
			logInfo("GeoIP database downloaded, verified, and updated successfully.")
		} else {
			logInfo("GeoIP database is identical to the existing one, no update needed.")
		}
	} else {
		logInfo("GeoIP database at %s is up to date.", dbPath)
	}

	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open GeoIP database: %v", err)
	}
	dbValue.Store(db)

	// Detect database type (City or Country)
	if err := initDatabaseType(db); err != nil {
		log.Fatalf("Failed to detect database type: %v", err)
	}

	// Start background goroutine for periodic database updates
	if updateIntervalHours > 0 {
		go periodicDatabaseUpdater(licenseKey, dbPath, updateIntervalHours)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/country/", countryHandler)
	mux.HandleFunc("/city/", cityHandler)
	mux.HandleFunc("/region/", regionHandler)
	mux.HandleFunc("/health", healthHandler)

	// Configure HTTP server with timeouts
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}

	// Channel to receive shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		logInfo("GeoIP API listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	logInfo("Shutdown signal received, initiating graceful shutdown...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server gracefully
	if err := server.Shutdown(ctx); err != nil {
		logError("HTTP server shutdown error: %v", err)
	} else {
		logInfo("HTTP server stopped gracefully")
	}

	// Cleanup database
	dbMutex.Lock()
	if dbRaw := dbValue.Load(); dbRaw != nil {
		if db, ok := dbRaw.(*geoip2.Reader); ok {
			db.Close()
			logInfo("GeoIP database closed")
		}
	}
	dbMutex.Unlock()

	logInfo("Shutdown complete")
}

func periodicDatabaseUpdater(licenseKey, dbPath string, intervalHours int) {
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	defer ticker.Stop()

	logInfo("Started periodic database updater (interval: %d hours)", intervalHours)

	for range ticker.C {
		logDebug("Periodic check triggered - checking if database needs to be updated...")

		fileInfo, err := os.Stat(dbPath)
		if err != nil {
			logError("Failed to get file info for %s: %v", dbPath, err)
			continue
		}

		lastModified := fileInfo.ModTime()
		dbAge := time.Since(lastModified)
		logDebug("Database age: %.1f hours (threshold: %d hours)", dbAge.Hours(), intervalHours)

		if dbAge > time.Duration(intervalHours)*time.Hour {
			logInfo("Database is older than %d hours, starting update...", intervalHours)

			if licenseKey == "" {
				logError("MAXMIND_LICENSE_KEY not set, skipping database update")
				continue
			}

			updated, err := downloadGeoLite2DB(licenseKey, dbPath)
			if err != nil {
				logError("Failed to update database: %v", err)
				continue
			}

			if !updated {
				logDebug("Database unchanged, skipping reload.")
				continue
			}

			logInfo("Database downloaded successfully, reloading...")
			if err := reloadDatabase(dbPath); err != nil {
				logError("Failed to reload database: %v", err)
				continue
			}

			logInfo("Database updated and reloaded successfully")
		} else {
			logDebug("Database is up to date (last modified: %s)", lastModified.Format(time.RFC3339))
		}
	}
}

func reloadDatabase(dbPath string) error {
	newDB, err := geoip2.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open new database: %w", err)
	}

	// Detect database type for the new database (without storing yet)
	newIsCityDB, err := isCityDatabase(newDB)
	if err != nil {
		newDB.Close()
		return fmt.Errorf("failed to detect new database type: %w", err)
	}

	// Acquire write lock to swap databases atomically
	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Atomically swap both the database and its type flag together
	oldDB := dbValue.Swap(newDB)
	isCityDB.Store(newIsCityDB)

	if newIsCityDB {
		logInfo("Reloaded GeoIP database type: City")
	} else {
		logInfo("Reloaded GeoIP database type: Country")
	}

	// Close old database if it exists
	if oldDB != nil {
		if oldReader, ok := oldDB.(*geoip2.Reader); ok {
			logInfo("Closing old GeoIP database.")
			oldReader.Close()
		}
	}

	return nil
}

// downloadGeoLite2DB downloads and verifies the GeoLite2 database.
// Returns (true, nil) if the database was updated, (false, nil) if it was already up-to-date,
// or (false, err) on failure.
func downloadGeoLite2DB(licenseKey, dbPath string) (bool, error) {
	// Determine which edition to download based on filename
	editionID := "GeoLite2-Country"
	if strings.Contains(strings.ToLower(dbPath), "city") {
		editionID = "GeoLite2-City"
	}

	logDebug("Starting database download from MaxMind (Edition: %s)", editionID)

	// Build URL with proper encoding
	downloadURL := fmt.Sprintf(
		"https://download.maxmind.com/app/geoip_download?edition_id=%s&license_key=%s&suffix=tar.gz",
		url.QueryEscape(editionID),
		url.QueryEscape(licenseKey),
	)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: httpTimeout,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return false, fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to download database: received status code %d, response: %s", resp.StatusCode, resp.Status)
	}

	logDebug("Download successful, extracting archive...")
	tmpDir, err := os.MkdirTemp("", "geoipdb")
	if err != nil {
		return false, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Limit the download size to prevent disk exhaustion
	limitedReader := io.LimitReader(resp.Body, maxDownloadSize)
	gzr, err := gzip.NewReader(limitedReader)
	if err != nil {
		return false, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var tempMMDBPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to read tar header: %w", err)
		}

		if strings.HasSuffix(header.Name, ".mmdb") {
			tempMMDBPath = filepath.Join(tmpDir, filepath.Base(header.Name))
			outFile, err := os.Create(tempMMDBPath)
			if err != nil {
				return false, fmt.Errorf("failed to create temporary .mmdb file: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return false, fmt.Errorf("failed to write to temporary .mmdb file: %w", err)
			}
			outFile.Close()
			break // Found the .mmdb file, no need to read further
		}
	}

	if tempMMDBPath == "" {
		return false, fmt.Errorf("could not find .mmdb file in archive")
	}

	// --- Verification Step 1: Load Test ---
	logDebug("Verifying downloaded database: %s", tempMMDBPath)
	verifiedDB, err := geoip2.Open(tempMMDBPath)
	if err != nil {
		return false, fmt.Errorf("verification failed: new database is invalid: %w", err)
	}

	// --- Verification Step 2: Lookup Test ---
	record, err := verifiedDB.Country(testIP)
	if err != nil {
		verifiedDB.Close()
		return false, fmt.Errorf("verification failed: lookup for %s failed on new database: %w", testIP, err)
	}
	if record.Country.IsoCode != "US" {
		logInfo("Warning: Test IP %s returned country %s, expected US. Continuing with update but this might indicate an issue.", testIP, record.Country.IsoCode)
	} else {
		logDebug("Verification successful: Test IP %s correctly identified as %s.", testIP, record.Country.IsoCode)
	}

	// Close the verification database before moving the file to prevent resource leaks
	verifiedDB.Close()

	// Compare hashes to avoid unnecessary replacement if the database hasn't changed
	if _, err := os.Stat(dbPath); err == nil {
		existingHash, err1 := fileSHA256(dbPath)
		newHash, err2 := fileSHA256(tempMMDBPath)
		if err1 == nil && err2 == nil && existingHash == newHash {
			logInfo("Downloaded database is identical to the current database, skipping update.")
			return false, nil
		}
	}

	// Ensure the destination directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	logDebug("Moving verified database from %s to %s", tempMMDBPath, dbPath)
	if err := os.Rename(tempMMDBPath, dbPath); err != nil {
		logDebug("Rename failed (possibly cross-device), falling back to copy: %v", err)
		src, err := os.Open(tempMMDBPath)
		if err != nil {
			return false, fmt.Errorf("failed to open source file for copy: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(dbPath)
		if err != nil {
			return false, fmt.Errorf("failed to create destination file for copy: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			os.Remove(dbPath)
			return false, fmt.Errorf("failed to copy database file: %w", err)
		}
		if err := dst.Sync(); err != nil {
			os.Remove(dbPath)
			return false, fmt.Errorf("failed to sync database file: %w", err)
		}
	}

	logDebug("Database file successfully updated at %s", dbPath)
	return true, nil
}

// fileSHA256 computes the SHA256 hash of a file and returns it as a hex string.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// getDatabase safely retrieves the database reader with proper locking.
// Returns the reader, a boolean indicating if it's a City database, and any error.
// The caller must call the returned unlock function when done.
func getDatabase() (*geoip2.Reader, bool, func(), error) {
	dbMutex.RLock()

	dbRaw := dbValue.Load()
	if dbRaw == nil {
		dbMutex.RUnlock()
		return nil, false, nil, fmt.Errorf("database not available")
	}

	db, ok := dbRaw.(*geoip2.Reader)
	if !ok {
		dbMutex.RUnlock()
		return nil, false, nil, fmt.Errorf("database not available")
	}

	isCity := isCityDB.Load()
	return db, isCity, dbMutex.RUnlock, nil
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	dbType := "Country"
	if isCityDB.Load() {
		dbType = "City"
	}

	fmt.Fprintf(w, `GeoIP API
Database Type: %s

Endpoints:
  /country/{ip}              - Returns country code only
  /city/{ip}                 - Returns country + city + region
  /region/{ip}               - Returns country + region
  /health                    - Health check

Response Formats:
  Add ?format=json for JSON response (default: plain text)

Examples:
  /country/8.8.8.8           -> US
  /country/8.8.8.8?format=json -> {"ip":"8.8.8.8","country":"US"}

  /city/8.8.8.8              -> US|Mountain View|CA
  /city/8.8.8.8?format=json  -> {"ip":"8.8.8.8","country":"US","city":"Mountain View","region":"CA"}

  /region/8.8.8.8            -> US|CA
  /region/8.8.8.8?format=json -> {"ip":"8.8.8.8","country":"US","region":"CA"}

Note: City and region data only available with GeoLite2-City database.
`, dbType)
}

type lookupResult struct {
	country string
	city    string
	region  string
}

func parseIPFromPath(r *http.Request, prefix string) (string, net.IP, error) {
	ipStr := strings.TrimPrefix(r.URL.Path, prefix)
	if ipStr == "" {
		return "", nil, fmt.Errorf("missing IP address")
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}
	return ipStr, ip, nil
}

func lookupIP(db *geoip2.Reader, isCity bool, ip net.IP, ipStr string) *lookupResult {
	result := &lookupResult{}
	if isCity {
		record, err := db.City(ip)
		if err != nil {
			logDebug("IP lookup failed for %s: %v", ipStr, err)
			result.country = "XX"
			return result
		}
		result.country = record.Country.IsoCode
		if result.country == "" {
			result.country = "XX"
		}
		result.city = record.City.Names["en"]
		if len(record.Subdivisions) > 0 {
			result.region = record.Subdivisions[0].IsoCode
		}
		logDebug("IP lookup: %s -> Country: %s, City: %s, Region: %s", ipStr, result.country, result.city, result.region)
	} else {
		record, err := db.Country(ip)
		if err != nil {
			logDebug("IP lookup failed for %s: %v", ipStr, err)
			result.country = "XX"
			return result
		}
		result.country = record.Country.IsoCode
		if result.country == "" {
			result.country = "XX"
		}
		logDebug("IP lookup: %s -> Country: %s", ipStr, result.country)
	}
	return result
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logError("Failed to encode JSON response: %v", err)
	}
}

func respondText(w http.ResponseWriter, parts ...string) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, strings.Join(parts, "|"))
}

func countryHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, ip, err := parseIPFromPath(r, "/country/")
	if err != nil {
		http.Error(w, "Usage: /country/{ip} or /country/{ip}?format=json", http.StatusBadRequest)
		return
	}

	db, isCity, unlock, err := getDatabase()
	if err != nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
	defer unlock()

	result := lookupIP(db, isCity, ip, ipStr)

	if r.URL.Query().Get("format") == "json" {
		respondJSON(w, CountryResponse{IP: ipStr, Country: result.country})
	} else {
		respondText(w, result.country)
	}
}

func cityHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, ip, err := parseIPFromPath(r, "/city/")
	if err != nil {
		http.Error(w, "Usage: /city/{ip} or /city/{ip}?format=json", http.StatusBadRequest)
		return
	}

	db, isCity, unlock, err := getDatabase()
	if err != nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
	defer unlock()

	result := lookupIP(db, isCity, ip, ipStr)

	if r.URL.Query().Get("format") == "json" {
		respondJSON(w, CityResponse{IP: ipStr, Country: result.country, City: result.city, Region: result.region})
	} else {
		respondText(w, result.country, result.city, result.region)
	}
}

func regionHandler(w http.ResponseWriter, r *http.Request) {
	ipStr, ip, err := parseIPFromPath(r, "/region/")
	if err != nil {
		http.Error(w, "Usage: /region/{ip} or /region/{ip}?format=json", http.StatusBadRequest)
		return
	}

	db, isCity, unlock, err := getDatabase()
	if err != nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}
	defer unlock()

	result := lookupIP(db, isCity, ip, ipStr)

	if r.URL.Query().Get("format") == "json" {
		respondJSON(w, RegionResponse{IP: ipStr, Country: result.country, Region: result.region})
	} else {
		respondText(w, result.country, result.region)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	db, _, unlock, err := getDatabase()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "ERROR: Database not loaded")
		return
	}
	defer unlock()

	_, err = db.Country(testIP)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "ERROR: Database lookup failed: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
