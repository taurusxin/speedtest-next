package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr      = ":8080"
	defaultChunkSize = 256 * 1024
	defaultBytes     = 8 * 1024 * 1024
	maxDownloadBytes = 256 * 1024 * 1024
	maxUploadBytes   = 128 * 1024 * 1024
)

//go:embed web/dist web/dist/**
var embeddedFrontend embed.FS

type phaseConfig struct {
	Concurrency int   `json:"concurrency"`
	DurationMS  int   `json:"durationMs"`
	ChunkBytes  int64 `json:"chunkBytes"`
}

type latencyConfig struct {
	SampleCount int `json:"sampleCount"`
	SampleGapMS int `json:"sampleGapMs"`
}

type runtimeConfig struct {
	APITargets struct {
		IPv4 string `json:"ipv4"`
		IPv6 string `json:"ipv6"`
	} `json:"apiTargets"`
	Latency                latencyConfig `json:"latency"`
	Download               phaseConfig   `json:"download"`
	Upload                 phaseConfig   `json:"upload"`
	SamplingIntervalMS     int           `json:"samplingIntervalMs"`
	ChartPointsLimit       int           `json:"chartPointsLimit"`
	DisplaySmoothingFactor float64       `json:"displaySmoothingFactor"`
	AllowedOrigins         []string      `json:"-"`
}

func main() {
	addr := envOrDefault("SPEEDTEST_ADDR", defaultAddr)
	cfg, err := loadRuntimeConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	handler := newServer(envOrDefault("SPEEDTEST_STATIC_DIR", ""), cfg)

	log.Printf("speedtest server listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func newServer(staticDir string, cfg runtimeConfig) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	mux.HandleFunc("/api/v1/latency", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	mux.HandleFunc("/api/v1/runtime-config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, cfg)
	})

	mux.HandleFunc("/api/v1/ip", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"ip": clientIP(r),
		})
	})

	mux.HandleFunc("/api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		bytesToWrite := clampInt(parseInt64(r.URL.Query().Get("bytes"), defaultBytes), 1, maxDownloadBytes)
		chunkSize := clampInt(parseInt64(r.URL.Query().Get("chunkSize"), defaultChunkSize), 4*1024, defaultChunkSize)
		serveDownload(w, r, bytesToWrite, chunkSize)
	})

	mux.HandleFunc("/api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		limited := http.MaxBytesReader(w, r.Body, maxUploadBytes)
		written, err := io.Copy(io.Discard, limited)
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"bytesReceived": written,
		})
	})

	frontendFS, err := resolveFrontendFS(staticDir)
	if err != nil {
		log.Printf("frontend unavailable: %v", err)
	}

	return withCORS(cfg.AllowedOrigins, withLogging(spaHandler(frontendFS, mux)))
}

func serveDownload(w http.ResponseWriter, r *http.Request, bytesToWrite int64, chunkSize int64) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Content-Length", strconv.FormatInt(bytesToWrite, 10))

	flusher, _ := w.(http.Flusher)
	buffer := makePatternBuffer(int(chunkSize))
	var written int64

	for written < bytesToWrite {
		if err := r.Context().Err(); err != nil {
			return
		}

		remaining := bytesToWrite - written
		next := int64(len(buffer))
		if remaining < next {
			next = remaining
		}

		count, err := w.Write(buffer[:next])
		written += int64(count)
		if err != nil {
			return
		}

		if flusher != nil {
			flusher.Flush()
		}
	}
}

func spaHandler(frontendFS fs.FS, apiHandler http.Handler) http.Handler {
	if frontendFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				apiHandler.ServeHTTP(w, r)
				return
			}

			http.Error(w, "frontend assets not found, build web app first", http.StatusServiceUnavailable)
		})
	}

	fileServer := http.FileServerFS(frontendFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath != "." && cleanPath != "" && fileExistsFS(frontendFS, cleanPath) {
			fileServer.ServeHTTP(w, r)
			return
		}

		indexFile, err := frontendFS.Open("index.html")
		if err == nil {
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, indexFile)
			return
		}

		http.Error(w, "frontend assets not found, build web app first", http.StatusServiceUnavailable)
	})
}

func withLogging(next http.Handler) http.Handler {
	noisyLoggingEnabled := envOrDefault("SPEEDTEST_LOG_NOISY_API", "false") == "true"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		if shouldSkipAccessLog(r, noisyLoggingEnabled) && recorder.statusCode < http.StatusBadRequest {
			return
		}

		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.statusCode, time.Since(started))
	})
}

func shouldSkipAccessLog(r *http.Request, noisyLoggingEnabled bool) bool {
	if noisyLoggingEnabled {
		return false
	}

	switch r.URL.Path {
	case "/api/v1/health", "/api/v1/runtime-config", "/api/v1/latency", "/api/v1/ip", "/api/v1/download", "/api/v1/upload":
		return true
	default:
		return r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api/")
	}
}

func withCORS(allowedOrigins []string, next http.Handler) http.Handler {
	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowedOriginSet[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowOrigin := len(allowedOriginSet) == 0
			if !allowOrigin {
				_, allowOrigin = allowedOriginSet[origin]
			}

			if allowOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func makePatternBuffer(size int) []byte {
	buffer := make([]byte, size)
	for index := range buffer {
		buffer[index] = byte((index*31 + 17) % 251)
	}
	return buffer
}

func parseInt64(raw string, fallback int) int64 {
	if raw == "" {
		return int64(fallback)
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return int64(fallback)
	}

	return value
}

func clampInt(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envInt64OrDefault(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func envFloatOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func loadRuntimeConfigFromEnv() (runtimeConfig, error) {
	cfg := runtimeConfig{}

	cfg.APITargets.IPv4 = strings.TrimSpace(os.Getenv("SPEEDTEST_TARGET_IPV4"))
	cfg.APITargets.IPv6 = strings.TrimSpace(os.Getenv("SPEEDTEST_TARGET_IPV6"))
	if cfg.APITargets.IPv4 == "" || cfg.APITargets.IPv6 == "" {
		return runtimeConfig{}, errors.New("SPEEDTEST_TARGET_IPV4 and SPEEDTEST_TARGET_IPV6 are required")
	}

	cfg.Latency.SampleCount = envIntOrDefault("SPEEDTEST_LATENCY_SAMPLE_COUNT", 10)
	cfg.Latency.SampleGapMS = envIntOrDefault("SPEEDTEST_LATENCY_SAMPLE_GAP_MS", 160)
	cfg.Download.Concurrency = envIntOrDefault("SPEEDTEST_DOWNLOAD_CONCURRENCY", 6)
	cfg.Download.DurationMS = envIntOrDefault("SPEEDTEST_DOWNLOAD_DURATION_MS", 9000)
	cfg.Download.ChunkBytes = envInt64OrDefault("SPEEDTEST_DOWNLOAD_CHUNK_BYTES", 6*1024*1024)
	cfg.Upload.Concurrency = envIntOrDefault("SPEEDTEST_UPLOAD_CONCURRENCY", 4)
	cfg.Upload.DurationMS = envIntOrDefault("SPEEDTEST_UPLOAD_DURATION_MS", 7000)
	cfg.Upload.ChunkBytes = envInt64OrDefault("SPEEDTEST_UPLOAD_CHUNK_BYTES", 1024*1024)
	cfg.SamplingIntervalMS = envIntOrDefault("SPEEDTEST_SAMPLING_INTERVAL_MS", 250)
	cfg.ChartPointsLimit = envIntOrDefault("SPEEDTEST_CHART_POINTS_LIMIT", 120)
	cfg.DisplaySmoothingFactor = envFloatOrDefault("SPEEDTEST_DISPLAY_SMOOTHING_FACTOR", 0.35)
	cfg.AllowedOrigins = parseCSVEnv("SPEEDTEST_ALLOWED_ORIGINS")

	return cfg, nil
}

func parseCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}

func clientIP(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func resolveFrontendFS(staticDir string) (fs.FS, error) {
	if staticDir != "" {
		if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
			return os.DirFS(staticDir), nil
		}
	}

	distFS, err := fs.Sub(embeddedFrontend, "web/dist")
	if err == nil {
		if _, statErr := fs.Stat(distFS, "index.html"); statErr == nil {
			return distFS, nil
		}
	}

	if staticDir != "" {
		return nil, os.ErrNotExist
	}

	return nil, err
}

func fileExistsFS(fileSystem fs.FS, target string) bool {
	info, err := fs.Stat(fileSystem, target)
	return err == nil && !info.IsDir()
}
