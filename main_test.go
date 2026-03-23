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

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	newServer(t.TempDir()).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestDownloadEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/download?bytes=4096&chunkSize=1024", nil)

	newServer(t.TempDir()).ServeHTTP(recorder, request)

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

	newServer(t.TempDir()).ServeHTTP(recorder, request)

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

	newServer(staticDir).ServeHTTP(recorder, request)

	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read spa body failed: %v", err)
	}

	if !strings.Contains(string(body), "ok") {
		t.Fatalf("expected fallback index, got %s", string(body))
	}
}
