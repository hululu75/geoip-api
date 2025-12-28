package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/oschwald/geoip2-golang"
)

var dbValue atomic.Value // stores *geoip2.Reader

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
		logInfo("Unknown LOG_LEVEL '%s', defaulting to INFO", logLevelStr)
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
		if i, err := strconv.Atoi(updateIntervalHoursStr); err == nil {
			updateIntervalHours = i
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
		if err := downloadGeoLite2DB(licenseKey, dbPath); err != nil {
			log.Fatalf("Failed to download or verify GeoIP database: %v", err)
		}
		logInfo("GeoIP database downloaded, verified, and updated successfully.")
	} else {
		logInfo("GeoIP database at %s is up to date.", dbPath)
	}

	db, err := geoip2.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open GeoIP database: %v", err)
	}
	dbValue.Store(db)

	// Start background goroutine for periodic database updates
	if updateIntervalHours > 0 {
		go periodicDatabaseUpdater(licenseKey, dbPath, updateIntervalHours)
	}

	// Cleanup on shutdown (best effort)
	defer func() {
		if db := dbValue.Load(); db != nil {
			db.(*geoip2.Reader).Close()
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/country/", countryHandler)
	http.HandleFunc("/health", healthHandler)

	logInfo("GeoIP API listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
		ageHours := time.Since(lastModified).Hours()
		logDebug("Database age: %.1f hours (threshold: %d hours)", ageHours, intervalHours)

		if time.Since(lastModified) > time.Duration(intervalHours)*time.Hour {
			logInfo("Database is older than %d hours, starting update...", intervalHours)

			if licenseKey == "" {
				logError("MAXMIND_LICENSE_KEY not set, skipping database update")
				continue
			}

			if err := downloadGeoLite2DB(licenseKey, dbPath); err != nil {
				logError("Failed to update database: %v", err)
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

	// Atomically swap the database
	oldDB := dbValue.Swap(newDB)

	// Close old database if it exists
	if oldDB != nil {
		if oldReader, ok := oldDB.(*geoip2.Reader); ok {
			oldReader.Close()
		}
	}

	return nil
}

func downloadGeoLite2DB(licenseKey, dbPath string) error {
	logDebug("Starting database download from MaxMind")
	url := fmt.Sprintf("https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=%s&suffix=tar.gz", licenseKey)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download database: received status code %d, response: %s", resp.StatusCode, resp.Status)
	}

	logDebug("Download successful, extracting archive...")
	tmpDir, err := os.MkdirTemp("", "geoipdb")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var mmdbFileName string
	var tempMMDBPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if strings.HasSuffix(header.Name, ".mmdb") {
			mmdbFileName = filepath.Base(header.Name)
			tempMMDBPath = filepath.Join(tmpDir, mmdbFileName)
			outFile, err := os.Create(tempMMDBPath)
			if err != nil {
				return fmt.Errorf("failed to create temporary .mmdb file: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write to temporary .mmdb file: %w", err)
			}
			outFile.Close()
			break // Found the .mmdb file, no need to read further
		}
	}

	if tempMMDBPath == "" {
		return fmt.Errorf("could not find .mmdb file in archive")
	}

	// --- Verification Step 1: Load Test ---
	logDebug("Verifying downloaded database: %s", tempMMDBPath)
	verifiedDB, err := geoip2.Open(tempMMDBPath)
	if err != nil {
		return fmt.Errorf("verification failed: new database is invalid: %w", err)
	}
	defer verifiedDB.Close()

	// --- Verification Step 2: Lookup Test ---
	testIP := net.ParseIP("8.8.8.8") // Google Public DNS, usually in US
	record, err := verifiedDB.Country(testIP)
	if err != nil {
		return fmt.Errorf("verification failed: lookup for %s failed on new database: %w", testIP, err)
	}
	if record.Country.IsoCode != "US" {
		logInfo("Warning: Test IP %s returned country %s, expected US. Continuing with update but this might indicate an issue.", testIP, record.Country.IsoCode)
	} else {
		logDebug("Verification successful: Test IP %s correctly identified as %s.", testIP, record.Country.IsoCode)
	}

	// Ensure the destination directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	// Atomically replace the database file
	logDebug("Moving verified database from %s to %s", tempMMDBPath, dbPath)
	if err := os.Rename(tempMMDBPath, dbPath); err != nil {
		return fmt.Errorf("failed to move verified database file from %s to %s: %w", tempMMDBPath, dbPath, err)
	}

	logDebug("Database file successfully updated at %s", dbPath)
	return nil
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
		logDebug("Invalid IP address requested: %s", ipStr)
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	// Lock-free atomic load - no performance impact!
	db := dbValue.Load().(*geoip2.Reader)
	record, err := db.Country(ip)

	if err != nil {
		logDebug("IP lookup failed for %s: %v", ipStr, err)
		country := "XX"
		respondWithFormat(w, r, ipStr, country)
		return
	}

	country := record.Country.IsoCode
	if country == "" {
		country = "XX"
	}

	logDebug("IP lookup: %s -> %s", ipStr, country)
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
