package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestParseIPFromPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		prefix    string
		wantIP    string
		wantEmpty bool
		wantErr   bool
	}{
		{"valid IPv4", "/country/8.8.8.8", "/country/", "8.8.8.8", false, false},
		{"valid IPv6", "/country/::1", "/country/", "::1", false, false},
		{"empty IP", "/country/", "/country/", "", true, true},
		{"invalid IP", "/country/abc", "/country/", "", false, true},
		{"valid with prefix", "/city/1.1.1.1", "/city/", "1.1.1.1", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			ipStr, ip, err := parseIPFromPath(req, tt.prefix)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ipStr != tt.wantIP {
				t.Errorf("ipStr = %q, want %q", ipStr, tt.wantIP)
			}
			if tt.wantEmpty {
				if ip != nil {
					t.Errorf("expected nil IP")
				}
			} else {
				if ip == nil || ip.String() != tt.wantIP {
					t.Errorf("ip = %v, want %s", ip, tt.wantIP)
				}
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, CountryResponse{IP: "8.8.8.8", Country: "US"})

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp CountryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.IP != "8.8.8.8" || resp.Country != "US" {
		t.Errorf("response = %+v, want ip=8.8.8.8 country=US", resp)
	}
}

func TestRespondText(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"single part", []string{"US"}, "US\n"},
		{"two parts", []string{"US", "CA"}, "US|CA\n"},
		{"three parts", []string{"US", "Mountain View", "CA"}, "US|Mountain View|CA\n"},
		{"empty parts", []string{"US", "", "CA"}, "US||CA\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			respondText(rec, tt.parts...)

			if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
				t.Errorf("Content-Type = %q, want text/plain", ct)
			}
			if got := rec.Body.String(); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRootHandler(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{"root path", "/", http.StatusOK},
		{"not found", "/foo", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			rootHandler(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestRootHandlerShowsDBType(t *testing.T) {
	origIsCity := isCityDB.Load()
	defer isCityDB.Store(origIsCity)

	isCityDB.Store(false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	rootHandler(rec, req)
	if !strings.Contains(rec.Body.String(), "Country") {
		t.Error("expected 'Country' in response body")
	}

	isCityDB.Store(true)
	rec = httptest.NewRecorder()
	rootHandler(rec, req)
	if !strings.Contains(rec.Body.String(), "City") {
		t.Error("expected 'City' in response body")
	}
}

func TestGetDatabaseNoDB(t *testing.T) {
	origDB := dbValue.Swap("dummy")
	if origDB != nil {
		defer dbValue.Store(origDB)
	}

	_, _, _, err := getDatabase()
	if err == nil {
		t.Error("expected error when database is not loaded")
	}
}

func TestCountryHandlerInvalidIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/country/invalid", nil)
	rec := httptest.NewRecorder()
	countryHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCountryHandlerEmptyIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/country/", nil)
	rec := httptest.NewRecorder()
	countryHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCityHandlerInvalidIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/city/notanip", nil)
	rec := httptest.NewRecorder()
	cityHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRegionHandlerInvalidIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/region/notanip", nil)
	rec := httptest.NewRecorder()
	regionHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCountryResponseJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, CountryResponse{IP: "1.2.3.4", Country: "DE"})

	var resp CountryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Country != "DE" {
		t.Errorf("country = %q, want %q", resp.Country, "DE")
	}
}

func TestCityResponseJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, CityResponse{IP: "1.2.3.4", Country: "US", City: "Denver", Region: "CO"})

	var resp CityResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.City != "Denver" || resp.Region != "CO" {
		t.Errorf("city=%q region=%q, want Denver/CO", resp.City, resp.Region)
	}
}

func TestCityResponseJSONEmptyFields(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, CityResponse{IP: "1.2.3.4", Country: "US"})

	var resp CityResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.City != "" || resp.Region != "" {
		t.Errorf("city=%q region=%q, want empty", resp.City, resp.Region)
	}
}

func TestRegionResponseJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, RegionResponse{IP: "1.2.3.4", Country: "US", Region: "CA"})

	var resp RegionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Region != "CA" {
		t.Errorf("region = %q, want %q", resp.Region, "CA")
	}
}

func TestDBMutexSerializesAccess(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dbMutex.RLock()
			dbMutex.RUnlock()
		}()
	}
	wg.Wait()
}

func TestAtomicDBState(t *testing.T) {
	isCityDB.Store(true)
	if !isCityDB.Load() {
		t.Error("expected isCityDB to be true")
	}
	isCityDB.Store(false)
	if isCityDB.Load() {
		t.Error("expected isCityDB to be false")
	}
}

