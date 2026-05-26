package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// Context key types for request-scoped values
type contextKey string

const (
	languageKey  contextKey = "language"
	jobStatusKey contextKey = "job_status"
)

// SetLanguage stores the language in the request context.
func SetLanguage(r *http.Request, language string) *http.Request {
	ctx := context.WithValue(r.Context(), languageKey, language)
	return r.WithContext(ctx)
}

// GetLanguage retrieves the language from the request context.
func GetLanguage(r *http.Request) string {
	lang, ok := r.Context().Value(languageKey).(string)
	if !ok {
		return ""
	}
	return lang
}

// SetJobStatus stores the job status in the request context.
func SetJobStatus(r *http.Request, status string) *http.Request {
	ctx := context.WithValue(r.Context(), jobStatusKey, status)
	return r.WithContext(ctx)
}

// GetJobStatus retrieves the job status from the request context.
func GetJobStatus(r *http.Request) string {
	status, ok := r.Context().Value(jobStatusKey).(string)
	if !ok {
		return ""
	}
	return status
}

// statusRecorder wraps http.ResponseWriter to capture the response status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code when WriteHeader is called
func (sr *statusRecorder) WriteHeader(status int) {
	sr.status = status
	sr.ResponseWriter.WriteHeader(status)
}

// generateRequestID creates a unique request ID as hex-encoded random bytes.
func generateRequestID() string {
	b := make([]byte, 16) // 128 bits
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Logger is a chi-compatible HTTP middleware that logs structured JSON per request.
// Each request generates one JSON log line with timestamp, request_id, method, path,
// status, duration, and optional language/job_status fields (for POST /run).
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := generateRequestID()

		// Store request ID in context for potential use by handlers
		ctx := context.WithValue(r.Context(), contextKey("request_id"), requestID)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK, // default if WriteHeader is never called
		}

		// Call the next handler
		next.ServeHTTP(recorder, r)

		// Calculate request duration
		duration := time.Since(start)

		// Extract request-scoped fields (set by handlers like POST /run)
		language := GetLanguage(r)
		jobStatus := GetJobStatus(r)

		// Log with structured attributes
		slog.LogAttrs(
			r.Context(),
			slog.LevelInfo,
			"http_request",
			slog.String("ts", start.Format(time.RFC3339)),
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.RequestURI),
			slog.Int("status", recorder.status),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("language", language),
			slog.String("job_status", jobStatus),
		)
	})
}
