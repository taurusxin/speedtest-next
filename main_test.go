package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRuntimeConfig() runtimeConfig {
	cfg := runtimeConfig{}
	cfg.APITargets.IPv4 = "speedtest-v4.example.com"
	cfg.APITargets.IPv6 = "speedtest-v6.example.com"
	cfg.Latency.SampleCount = 10
	cfg.Latency.SampleGapMS = 160
	cfg.Download.Concurrency = 6
	cfg.Download.DurationMS = 9000
	cfg.Download.ChunkBytes = 6 * 1024 * 1024
	cfg.Upload.Concurrency = 4
	cfg.Upload.DurationMS = 7000
	cfg.Upload.ChunkBytes = 1024 * 1024
	cfg.SamplingIntervalMS = 250
	cfg.ChartPointsLimit = 120
	cfg.DisplaySmoothingFactor = 0.35
	return cfg
}

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	newServer(t.TempDir(), testRuntimeConfig()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestDownloadEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/download?bytes=4096&chunkSize=1024", nil)

	newServer(t.TempDir(), testRuntimeConfig()).ServeHTTP(recorder, request)

	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read download body failed: %v", err)
	}

	if len(body) != 4096 {
		t.Fatalf("expected 4096 bytes, got %d", len(body))
	}
}

func TestUploadEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/upload", strings.NewReader(strings.Repeat("x", 1024)))
	request.Header.Set("Content-Type", "application/octet-stream")

	newServer(t.TempDir(), testRuntimeConfig()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestSPAFallback(t *testing.T) {
	staticDir := t.TempDir()
	indexPath := filepath.Join(staticDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatalf("write index failed: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	newServer(staticDir, testRuntimeConfig()).ServeHTTP(recorder, request)

	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read spa body failed: %v", err)
	}

	if !strings.Contains(string(body), "ok") {
		t.Fatalf("expected fallback index, got %s", string(body))
	}
}

func TestRuntimeConfigEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil)

	newServer(t.TempDir(), testRuntimeConfig()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read runtime config body failed: %v", err)
	}

	if !strings.Contains(string(body), "speedtest-v4.example.com") {
		t.Fatalf("expected runtime config response, got %s", string(body))
	}
}
