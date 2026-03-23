package main

import (
	"embed"
	"encoding/json"
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

func main() {
	addr := envOrDefault("SPEEDTEST_ADDR", defaultAddr)
	handler := newServer(envOrDefault("SPEEDTEST_STATIC_DIR", ""))

	log.Printf("speedtest server listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func newServer(staticDir string) http.Handler {
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

	return withCORS(withLogging(spaHandler(frontendFS, mux)))
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
	case "/api/v1/health", "/api/v1/latency", "/api/v1/ip", "/api/v1/download", "/api/v1/upload":
		return true
	default:
		return r.Method == http.MethodOptions && strings.HasPrefix(r.URL.Path, "/api/")
	}
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigins := map[string]struct{}{
		"https://speedtest.taurusxin.com":    {},
		"https://v4-speedtest.taurusxin.com": {},
		"https://v6-speedtest.taurusxin.com": {},
		"http://localhost:5173":              {},
		"http://127.0.0.1:5173":              {},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
